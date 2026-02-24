// Package data provides the data models and database interaction logic
// for the library management system.
package data

import "time"

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