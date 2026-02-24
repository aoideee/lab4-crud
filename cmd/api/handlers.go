// cmd/api/handlers.go
// This file contains all HTTP request handlers for the books resource.
// Each handler is a method on *applicationDependencies so it has access
// to the logger, models, and config without global variables.
package main

import (
	"errors"
	"net/http"

	"github.com/aoideee/lab4-tyshadaniels/internal/data"
	"github.com/aoideee/lab4-tyshadaniels/internal/validator"
)

// createBookHandler handles POST /v1/books.
// It reads a JSON body, validates all fields with a Validator, inserts the record,
// and responds with 201 Created plus the fully-populated book.
func (app *applicationDependencies) createBookHandler(w http.ResponseWriter, r *http.Request) {
	var input data.CreateBookInput

	// Decode the request body safely (1MB cap, no unknown fields).
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// --- Validation ---
	v := validator.New()
	v.Check(input.Title != "", "title", "must be provided")
	v.Check(len(input.Title) <= 255, "title", "must not be more than 255 characters long")
	v.Check(input.ISBN != "", "isbn", "must be provided")
	v.Check(len(input.ISBN) == 13, "isbn", "must be exactly 13 characters long")
	v.Check(input.Publisher != "", "publisher", "must be provided")
	v.Check(input.PublicationYear > 0, "publication_year", "must be provided")
	v.Check(input.PublicationYear <= 2026, "publication_year", "must not be in the future")
	v.Check(input.MinimumAge >= 0, "minimum_age", "must be zero or greater")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Map the validated input onto a new Book struct.
	book := &data.Book{
		Title:           input.Title,
		ISBN:            input.ISBN,
		Publisher:       input.Publisher,
		PublicationYear: input.PublicationYear,
		MinimumAge:      input.MinimumAge,
		Description:     input.Description,
	}

	// Persist the book; Insert() writes the auto-generated ID and timestamps back.
	err = app.models.Books.Insert(book)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with the created book and 201 Created.
	err = app.writeJSON(w, http.StatusCreated, envelope{"book": book}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// showBookHandler handles GET /v1/books/:id.
// It calls Get(id) directly on the model — no full table scan needed.
func (app *applicationDependencies) showBookHandler(w http.ResponseWriter, r *http.Request) {
	// Extract and validate the :id URL parameter.
	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Fetch the single record from the database by primary key.
	book, err := app.models.Books.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"book": book}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listBooksHandler handles GET /v1/books.
// It reads optional page, page_size, and sort query parameters, validates them,
// and returns a paginated list of books together with pagination metadata.
func (app *applicationDependencies) listBooksHandler(w http.ResponseWriter, r *http.Request) {
	// The struct we will fill from the URL query string.
	var queryInput struct {
		Page     int
		PageSize int
		Sort     string
	}

	// Read query parameters with sensible defaults.
	qs := r.URL.Query()
	queryInput.Page = app.readInt(qs, "page", 1)
	queryInput.PageSize = app.readInt(qs, "page_size", 10)
	queryInput.Sort = app.readString(qs, "sort", "book_id")

	// --- Validation ---
	v := validator.New()
	v.Check(queryInput.Page > 0, "page", "must be greater than zero")
	v.Check(queryInput.Page <= 10_000_000, "page", "must be a maximum of 10 million")
	v.Check(queryInput.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(queryInput.PageSize <= 100, "page_size", "must be a maximum of 100")
	v.Check(validator.In(queryInput.Sort, "book_id", "title", "publication_year", "-book_id", "-title", "-publication_year"),
		"sort", "invalid sort value")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Build the Filters value to pass to GetAll.
	filters := data.Filters{
		Page:     queryInput.Page,
		PageSize: queryInput.PageSize,
		Sort:     queryInput.Sort,
		SortSafeList: []string{
			"book_id", "title", "publication_year",
			"-book_id", "-title", "-publication_year",
		},
	}

	books, metadata, err := app.models.Books.GetAll(filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Include both the books and the pagination metadata in the response envelope.
	err = app.writeJSON(w, http.StatusOK, envelope{"books": books, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// replaceBookHandler handles PUT /v1/books/:id.
// PUT is a FULL replacement — the client must supply every field.
// If any required field is missing the request is rejected with 422.
// Use PATCH (/v1/books/:id) instead if you only want to update specific fields.
func (app *applicationDependencies) replaceBookHandler(w http.ResponseWriter, r *http.Request) {
	// Extract and validate the :id URL parameter.
	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Confirm the book exists before replacing it.
	book, err := app.models.Books.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Decode the complete replacement body. We reuse CreateBookInput because
	// PUT requires every field to be provided (same required fields as a create).
	var input data.CreateBookInput
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// --- Validation: all fields are required for a full replacement ---
	v := validator.New()
	v.Check(input.Title != "", "title", "must be provided")
	v.Check(len(input.Title) <= 255, "title", "must not be more than 255 characters long")
	v.Check(input.ISBN != "", "isbn", "must be provided")
	v.Check(len(input.ISBN) == 13, "isbn", "must be exactly 13 characters long")
	v.Check(input.Publisher != "", "publisher", "must be provided")
	v.Check(input.PublicationYear > 0, "publication_year", "must be provided")
	v.Check(input.PublicationYear <= 2026, "publication_year", "must not be in the future")
	v.Check(input.MinimumAge >= 0, "minimum_age", "must be zero or greater")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Overwrite all fields on the existing book record.
	book.Title = input.Title
	book.ISBN = input.ISBN
	book.Publisher = input.Publisher
	book.PublicationYear = input.PublicationYear
	book.MinimumAge = input.MinimumAge
	book.Description = input.Description

	// Persist the replaced book.
	err = app.models.Books.Update(book)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with the fully-replaced book.
	err = app.writeJSON(w, http.StatusOK, envelope{"book": book}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateBookHandler handles PATCH /v1/books/:id.
// It fetches the existing record with Get(id), applies only the non-nil input
// fields, validates the result, and saves the changes with Update().
func (app *applicationDependencies) updateBookHandler(w http.ResponseWriter, r *http.Request) {
	// Extract and validate the :id URL parameter.
	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Fetch the existing record directly by primary key — no table scan.
	book, err := app.models.Books.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Decode the partial update from the request body.
	var input data.UpdateBookInput
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Apply only the fields that were actually provided (non-nil pointers).
	if input.Title != nil {
		book.Title = *input.Title
	}
	if input.ISBN != nil {
		book.ISBN = *input.ISBN
	}
	if input.Publisher != nil {
		book.Publisher = *input.Publisher
	}
	if input.PublicationYear != nil {
		book.PublicationYear = *input.PublicationYear
	}
	if input.MinimumAge != nil {
		book.MinimumAge = *input.MinimumAge
	}
	if input.Description != nil {
		book.Description = *input.Description
	}

	// --- Validation on the merged (existing + updated) values ---
	v := validator.New()
	v.Check(book.Title != "", "title", "must be provided")
	v.Check(len(book.Title) <= 255, "title", "must not be more than 255 characters long")
	v.Check(book.ISBN != "", "isbn", "must be provided")
	v.Check(len(book.ISBN) == 13, "isbn", "must be exactly 13 characters long")
	v.Check(book.Publisher != "", "publisher", "must be provided")
	v.Check(book.PublicationYear > 0, "publication_year", "must be provided")
	v.Check(book.PublicationYear <= 2026, "publication_year", "must not be in the future")
	v.Check(book.MinimumAge >= 0, "minimum_age", "must be zero or greater")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Persist the changes.
	err = app.models.Books.Update(book)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with the updated book.
	err = app.writeJSON(w, http.StatusOK, envelope{"book": book}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteBookHandler handles DELETE /v1/books/:id.
// It deletes the matching record and responds with a success message.
// Returns 404 if no book with that ID exists.
func (app *applicationDependencies) deleteBookHandler(w http.ResponseWriter, r *http.Request) {
	// Extract and validate the :id URL parameter.
	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	err = app.models.Books.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "book successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
