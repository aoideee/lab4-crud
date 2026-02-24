// cmd/api/server.go
// This file contains the serve() method which starts the HTTP server and
// handles graceful shutdown when an OS signal is received.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// serve builds the HTTP server, starts it in a background goroutine, then
// blocks until it receives a SIGINT or SIGTERM signal. On signal receipt it
// initiates a graceful shutdown: in-flight requests are given 20 seconds to
// complete before the server is forcefully stopped.
func (app *applicationDependencies) serve() error {
	// Configure the HTTP server.
	apiServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// shutdownErr receives any error returned by Shutdown().
	shutdownErr := make(chan error)

	// Background goroutine: wait for a shutdown signal then gracefully stop.
	go func() {
		// quit is a buffered channel so the signal package never blocks.
		quit := make(chan os.Signal, 1)

		// Notify quit on SIGINT (Ctrl+C) and SIGTERM (kill / Docker stop).
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Block until a signal arrives.
		s := <-quit
		app.logger.Info("shutting down server", "signal", s.String())

		// Create a context with a 20-second timeout. Active requests must
		// complete within this window or they will be abandoned.
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// Shutdown stops accepting new connections and waits for active
		// requests to finish, respecting the context deadline.
		shutdownErr <- apiServer.Shutdown(ctx)
	}()

	// Start the server. ListenAndServe always returns a non-nil error; we
	// treat ErrServerClosed as normal (it means Shutdown was called).
	app.logger.Info("starting server", "address", apiServer.Addr, "environment", app.config.environment)

	err := apiServer.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Wait for the shutdown goroutine to finish and collect its error.
	err = <-shutdownErr
	if err != nil {
		return err
	}

	app.logger.Info("server stopped", "address", apiServer.Addr)
	return nil
}
