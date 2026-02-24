// internal/data/models.go
package data

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
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

// ErrRecordNotFound is returned when a query finds no matching row.
var ErrRecordNotFound = errors.New("record not found")

// Filters holds pagination and sorting parameters extracted from URL query strings.
type Filters struct {
	Page         int      // Current page number (1-indexed)
	PageSize     int      // Number of records per page
	Sort         string   // Column name to sort by (prefix with "-" for DESC)
	SortSafeList []string // Allowed sort columns to prevent SQL injection
}

// sortColumn returns the validated column name for ORDER BY, defaulting to book_id.
func (f Filters) sortColumn() string {
	for _, safe := range f.SortSafeList {
		if f.Sort == safe {
			return strings.TrimPrefix(f.Sort, "-")
		}
	}
	return "book_id" // safe fallback
}

// sortDirection returns "ASC" or "DESC" based on the Sort prefix.
func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

// limit returns the SQL LIMIT value derived from PageSize.
func (f Filters) limit() int { return f.PageSize }

// offset returns the SQL OFFSET value derived from Page and PageSize.
func (f Filters) offset() int { return (f.Page - 1) * f.PageSize }

// Metadata contains pagination information returned alongside list responses.
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

// calculateMetadata computes page metadata from total record count and filter values.
func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}
	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
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