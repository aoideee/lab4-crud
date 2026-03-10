// cmd/api/server.go
// This file contains the serve() method which starts the HTTP server and
// handles graceful shutdown when an OS signal is received.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// serve starts the HTTP server and waits for a shutdown signal (SIGINT/SIGTERM).
// When signaled, it gracefully shuts down with a 20-second timeout for in-flight requests.
func (app *applicationDependencies) serve() error {
	// Configure the HTTP server.
	apiServer := &http.Server {
        Addr: fmt.Sprintf(":%d", app.config.port),
        Handler: app.routes(),
        IdleTimeout: time.Minute,
        ReadTimeout: 5 * time.Second,
        WriteTimeout: 10 * time.Second,
        ErrorLog: slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
    }

	// shutdownErr receives errors from the shutdown goroutine.
	shutdownErr := make(chan error)

	// Background goroutine: wait for a shutdown signal then gracefully stop.
	go func() {
		// quit is buffered so the signal package never blocks.
		quit := make(chan os.Signal, 1)

		// Listen for SIGINT (Ctrl+C) and SIGTERM (kill / Docker stop).
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Block until a signal arrives.
		s := <-quit
		app.logger.Info("shutting down server", "signal", s.String())

		// Create a context with a 20-second timeout for active requests.
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// Initiate graceful shutdown.
		shutdownErr <- apiServer.Shutdown(ctx)
	}()

	// Start the server. ListenAndServe always returns a non-nil error.
	app.logger.Info("starting server", "address", apiServer.Addr, "environment", app.config.environment)

	err := apiServer.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Wait for the shutdown goroutine to finish.
	err = <-shutdownErr
	if err != nil {
		return err
	}

	time.Sleep(3 * time.Second)

	app.logger.Info("server stopped", "address", apiServer.Addr)
	return nil
}
