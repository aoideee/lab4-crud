// internal/data/models.go
package data

import (
	"database/sql"
)

// Models is a top-level container that groups all database model types together.
// It is passed around the application via applicationDependencies so every handler
// has access to the database without importing sql directly.
type Models struct {
	Books BookModel // Handles all database operations for the books table
}

// NewModels constructs a Models value wired up to the given database connection pool.
// Call this once during application startup and store the result in applicationDependencies.
func NewModels(db *sql.DB) Models {
	return Models{
		Books: BookModel{DB: db},
	}
}