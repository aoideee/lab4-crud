// internal/data/models.go
package data

import (
	"database/sql"
	"fmt"
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

// BookModel wraps a *sql.DB connection and provides methods for
// creating, reading, updating, and deleting book records.
type BookModel struct {
	DB *sql.DB // Shared database connection pool
}

// Insert adds a new book record to the database.
// After a successful insert, the database-assigned book_id, created_at, and
// updated_at values are written back into the book struct.
func (m BookModel) Insert(book *Book) error {
	query := `
        INSERT INTO books (title, isbn, publisher, publication_year, minimum_age, description)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING book_id, created_at, updated_at
    `

	// Run the INSERT and scan the auto-generated columns back into the struct.
	err := m.DB.QueryRow(
		query,
		book.Title,
		book.ISBN,
		book.Publisher,
		book.PublicationYear,
		book.MinimumAge,
		book.Description,
	).Scan(&book.ID, &book.CreatedAt, &book.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

// GetAll retrieves every book from the database, ordered by book_id ascending.
// Returns a slice of pointers so callers can modify records without extra copies.
func (m BookModel) GetAll() ([]*Book, error) {
	query := `
        SELECT book_id, title, isbn, publisher, publication_year, minimum_age, description, created_at, updated_at
        FROM books
        ORDER BY book_id`

	// Execute the SELECT and get a result set (rows).
	rows, err := m.DB.Query(query)
	if err != nil {
		return nil, err
	}
	// Always close the result set when we are done to free the database connection.
	defer rows.Close()

	books := []*Book{}

	// Iterate over each row and scan the columns into a Book struct.
	for rows.Next() {
		var book Book
		err := rows.Scan(
			&book.ID,
			&book.Title,
			&book.ISBN,
			&book.Publisher,
			&book.PublicationYear,
			&book.MinimumAge,
			&book.Description,
			&book.CreatedAt,
			&book.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		books = append(books, &book)
	}

	// Check for any error that occurred while iterating the rows.
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return books, nil
}

// Delete removes the book with the given id from the database.
// Returns an error if the id is invalid or if no matching record exists.
func (m BookModel) Delete(id int64) error {
	// Guard against obviously bad IDs before touching the database.
	if id < 1 {
		return fmt.Errorf("invalid ID")
	}

	query := `DELETE FROM books WHERE book_id = $1`

	// Exec returns a Result that tells us how many rows were affected.
	result, err := m.DB.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows were deleted, the book didn't exist.
	if rowsAffected == 0 {
		return fmt.Errorf("record not found")
	}

	return nil
}

// Update saves the modified fields of book back to the database.
// The WHERE clause matches on book.ID, and the database automatically
// updates the updated_at timestamp, which is scanned back into the struct.
func (m BookModel) Update(book *Book) error {
	query := `
		UPDATE books 
		SET title = $1, isbn = $2, publisher = $3, publication_year = $4, 
            minimum_age = $5, description = $6, updated_at = CURRENT_TIMESTAMP
		WHERE book_id = $7
		RETURNING updated_at`

	// Collect all arguments in order matching the $N placeholders above.
	args := []any{
		book.Title,
		book.ISBN,
		book.Publisher,
		book.PublicationYear,
		book.MinimumAge,
		book.Description,
		book.ID,
	}

	// Execute the UPDATE and scan the refreshed updated_at back into the struct.
	return m.DB.QueryRow(query, args...).Scan(&book.UpdatedAt)
}