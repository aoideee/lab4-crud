// Package data provides the data models and database interaction logic
// for the library management system.
package data

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Book represents a single book record stored in the database.
// It maps directly to a row in the "books" table.
type Book struct {
	ID              int64     `json:"book_id"`        // Unique identifier assigned by the database
	Title           string    `json:"title"`           // Title of the book
	ISBN            string    `json:"isbn"`            // 13-digit ISBN identifier
	Publisher       string    `json:"publisher"`       // Name of the publishing company
	PublicationYear int       `json:"publication_year"` // Year the book was published
	MinimumAge      int       `json:"minimum_age"`     // Minimum recommended reader age
	Description     string    `json:"description,omitempty"` // Optional short description (omitted from JSON if empty)
	CreatedAt       time.Time `json:"created_at"`     // Timestamp when the record was created
	UpdatedAt       time.Time `json:"updated_at"`     // Timestamp when the record was last modified
}

// CreateBookInput holds the fields a client must supply when creating a new book.
// All fields except Description are required.
type CreateBookInput struct {
	Title           string `json:"title"           validate:"required"`
	ISBN            string `json:"isbn"            validate:"required,len=13"`
	Publisher       string `json:"publisher"       validate:"required"`
	PublicationYear int    `json:"publication_year" validate:"required"`
	MinimumAge      int    `json:"minimum_age"     validate:"required"`
	Description     string `json:"description,omitempty"`
}

// UpdateBookInput holds the fields a client may supply when partially updating a book.
// Every field is a pointer so we can distinguish between "not provided" (nil)
// and "intentionally set to zero/empty". Only non-nil fields are applied.
type UpdateBookInput struct {
	Title           *string `json:"title"`
	ISBN            *string `json:"isbn"             validate:"omitempty,len=13"`
	Publisher       *string `json:"publisher"`
	PublicationYear *int    `json:"publication_year" validate:"omitempty,lte=2026"`
	MinimumAge      *int    `json:"minimum_age"      validate:"omitempty,min=0"`
	Description     *string `json:"description"`
}

// ErrRecordNotFound is returned when a query finds no matching row.
var ErrRecordNotFound = errors.New("record not found")

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

// Get retrieves a single book by its primary key.
// Returns ErrRecordNotFound if no book with the given id exists.
func (m BookModel) Get(id int64) (*Book, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT book_id, title, isbn, publisher, publication_year, minimum_age, description, created_at, updated_at
		FROM books
		WHERE book_id = $1`

	var book Book
	err := m.DB.QueryRow(query, id).Scan(
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
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &book, nil
}

// GetAll retrieves a paginated, sorted list of books.
// It uses a COUNT(*) OVER() window function so only one round-trip is needed.
// Returns the book slice and pagination Metadata.
func (m BookModel) GetAll(filters Filters) ([]*Book, Metadata, error) {
	// Build query dynamically using the validated sort column and direction.
	query := fmt.Sprintf(`
		SELECT count(*) OVER(), book_id, title, isbn, publisher, publication_year, minimum_age, description, created_at, updated_at
		FROM books
		ORDER BY %s %s, book_id ASC
		LIMIT $1 OFFSET $2`, filters.sortColumn(), filters.sortDirection())

	// Execute the SELECT and get a result set (rows).
	rows, err := m.DB.Query(query, filters.limit(), filters.offset())
	if err != nil {
		return nil, Metadata{}, err
	}
	// Always close the result set when we are done to free the database connection.
	defer rows.Close()

	totalRecords := 0
	books := []*Book{}

	// Iterate over each row and scan the columns into a Book struct.
	for rows.Next() {
		var book Book
		err := rows.Scan(
			&totalRecords, // COUNT(*) OVER() – same value on every row
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
			return nil, Metadata{}, err
		}
		books = append(books, &book)
	}

	// Check for any error that occurred while iterating the rows.
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	return books, metadata, nil
}

// Delete removes the book with the given id from the database.
// Returns ErrRecordNotFound if no matching record exists.
func (m BookModel) Delete(id int64) error {
	// Guard against obviously bad IDs before touching the database.
	if id < 1 {
		return ErrRecordNotFound
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
		return ErrRecordNotFound
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