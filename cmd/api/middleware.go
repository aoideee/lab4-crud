// cmd/api/middleware.go
// This file contains HTTP middleware used to wrap the router.
// Middleware functions intercept every request before it reaches a handler.
package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// recoverPanic catches any runtime panic that occurs in a downstream handler.
// Without this, a panic would cause the goroutine to terminate and the client's
// connection to be dropped silently. With this middleware the client receives a
// clean 500 Internal Server Error instead.
func (app *applicationDependencies) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// defer runs when the surrounding goroutine unwinds, even after a panic.
		defer func() {
			if err := recover(); err != nil {
				// Tell the HTTP server to close the connection after this response.
				w.Header().Set("Connection", "close")
				// Convert the recovered panic value to an error and send a 500.
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// client holds a per-IP rate limiter and the time it was last seen.
// lastSeen lets us evict old entries so the map does not grow forever.
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimit implements per-IP token-bucket rate limiting using the
// golang.org/x/time/rate package. Each unique IP gets its own limiter
// seeded with 2 tokens per second and a burst capacity of 4.
// A background goroutine cleans up entries that have not been seen in 3 minutes.
func (app *applicationDependencies) rateLimit(next http.Handler) http.Handler {
	// clients maps IP addresses to their individual rate limiters.
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Cleanup goroutine: remove stale IP entries every minute.
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract just the IP from the RemoteAddr (strips the port).
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		mu.Lock()
		// Create a new limiter for this IP if we have not seen it before.
		if _, found := clients[ip]; !found {
			clients[ip] = &client{
				limiter: rate.NewLimiter(rate.Limit(2), 4), // 2 req/s, burst of 4
			}
		}
		clients[ip].lastSeen = time.Now()

		// Allow() consumes one token; returns false if the bucket is empty.
		if !clients[ip].limiter.Allow() {
			mu.Unlock()
			app.rateLimitExceededResponse(w, r)
			return
		}
		mu.Unlock()

		next.ServeHTTP(w, r)
	})
}
