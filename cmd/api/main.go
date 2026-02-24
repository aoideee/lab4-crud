// Package main is the entry point for the library API server.
// It wires together configuration, the database connection, and the HTTP router.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aoideee/lab4-tyshadaniels/internal/data"

	_ "github.com/lib/pq" // Register the PostgreSQL driver with database/sql.
)

// appVersion is the current version of the API, shown in logs.
const appVersion = "1.0.0"

// serverConfig holds all the values that can be tweaked at startup via command-line flags.
type serverConfig struct {
	port        int    // TCP port the HTTP server listens on (default 4000)
	environment string // Runtime environment: development, staging, or production
	db          struct {
		dsn string // PostgreSQL Data Source Name (connection string)
	}
}

// applicationDependencies bundles every shared resource that HTTP handlers need.
// A pointer to this struct is passed as the receiver on all handler and route methods.
type applicationDependencies struct {
	config serverConfig // Server configuration loaded from flags
	logger *slog.Logger // Structured logger that writes to stdout
	models data.Models  // Database model layer for all tables
}

// main is the application entry point.
// It parses flags, opens the database, wires up dependencies, and starts the HTTP server.
func main() {
	var settings serverConfig

	// Register command-line flags so operators can override defaults at runtime.
	flag.IntVar(&settings.port, "port", 4000, "Server port")
	flag.StringVar(&settings.environment, "env", "development", "Environment(development|staging|production)")
	flag.StringVar(&settings.db.dsn, "db-dsn", "postgres://clms:clms@localhost/clms?sslmode=disable", "PostgreSQL DSN")

	flag.Parse()

	// Create a structured logger that writes human-readable text to stdout.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Open and verify the database connection pool.
	db, err := openDB(settings)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close() // Close the pool cleanly when main() returns.

	logger.Info("database connection pool established")

	// Bundle all shared dependencies into a single struct.
	appInstance := &applicationDependencies{
		config: settings,
		logger: logger,
		models: data.NewModels(db),
	}

	// Configure and create the HTTP server.
	apiServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", settings.port),
		Handler:      appInstance.routes(), // All routes are registered here.
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "address", apiServer.Addr, "environment", settings.environment)

	// ListenAndServe blocks until the server shuts down or encounters a fatal error.
	err = apiServer.ListenAndServe()
	logger.Error(err.Error())
	os.Exit(1)
}

// openDB opens a PostgreSQL connection pool using the DSN stored in settings,
// then pings the database with a 5-second timeout to confirm it is reachable.
// Returns the pool on success, or an error if the connection cannot be established.
func openDB(settings serverConfig) (*sql.DB, error) {
	// sql.Open only validates the DSN format; it does not actually connect yet.
	db, err := sql.Open("postgres", settings.db.dsn)
	if err != nil {
		return nil, err
	}

	// Create a context that cancels automatically after 5 seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// PingContext performs a real round-trip to verify the database is reachable.
	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}