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

// rateLimit is a middleware that limits the number of requests a client can make
// in a given time period. It uses the golang.org/x/time/rate package to implement
// a token bucket algorithm for rate limiting.
func (app *applicationDependencies) rateLimit(next http.Handler) http.Handler {
	// client holds a per-IP rate limiter and the time it was last seen.
	// lastSeen lets us evict old entries so the map does not grow forever.
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// background goroutine to remove clients that haven't been seen for more than 3 minutes.
	// This prevents the map from growing indefinitely if a client makes a single request and then disappears.
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.config.limiter.enabled {
			// Extract the client's IP address.
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}

			mu.Lock()

			// If the IP address is not already in the clients map, create a new client with a rate limiter.
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
				}
			}
			clients[ip].lastSeen = time.Now()

			// Check if the client's rate limiter allows the request. If not, send a 429 Too Many Requests response.
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}
			mu.Unlock()
		}

		next.ServeHTTP(w, r)
	})
}