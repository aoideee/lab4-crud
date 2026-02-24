// cmd/api/handlers.go
// This file contains all HTTP request handlers for the books resource.
// Each handler is a method on *applicationDependencies so it has access
// to the logger and database models.
package main

import (
	"net/http"

	"github.com/aoideee/lab4-tyshadaniels/internal/data"
)

// createBookHandler handles POST /v1/books.
// It reads a JSON body containing the new book's details, inserts a record
// into the database, and responds with the created book (including its
// database-assigned ID and timestamps) and a 201 Created status.
func (app *applicationDependencies) createBookHandler(w http.ResponseWriter, r *http.Request) {
	var input data.CreateBookInput

	// Decode the incoming JSON body into our input struct using the safe readJSON helper.
	// readJSON enforces a 1MB limit, rejects unknown fields, and ensures a single value.
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Map the validated input fields onto a new Book struct.
	book := &data.Book{
		Title:           input.Title,
		ISBN:            input.ISBN,
		Publisher:       input.Publisher,
		PublicationYear: input.PublicationYear,
		MinimumAge:      input.MinimumAge,
		Description:     input.Description,
	}

	// Persist the book to the database.
	// Insert() also writes the auto-generated ID and timestamps back into book.
	err = app.models.Books.Insert(book)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with the fully-populated book and a 201 Created status.
	err = app.writeJSON(w, http.StatusCreated, envelope{"book": book}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// showBookHandler handles GET /v1/books/:id.
// It parses the :id URL parameter, fetches all books, and returns the one
// whose ID matches. Responds 404 if no book with that ID exists.
func (app *applicationDependencies) showBookHandler(w http.ResponseWriter, r *http.Request) {
	// readIDParam extracts and validates the :id URL parameter.
	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Retrieve all books and scan for the requested ID.
	books, err := app.models.Books.GetAll()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	for _, b := range books {
		if b.ID == id {
			err = app.writeJSON(w, http.StatusOK, envelope{"book": b}, nil)
			if err != nil {
				app.serverErrorResponse(w, r, err)
			}
			return
		}
	}

	// No book matched the requested ID.
	app.notFoundResponse(w, r)
}

// listBooksHandler handles GET /v1/books.
// It fetches every book from the database and returns them as a JSON array.
func (app *applicationDependencies) listBooksHandler(w http.ResponseWriter, r *http.Request) {
	books, err := app.models.Books.GetAll()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"books": books}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateBookHandler handles PATCH /v1/books/:id.
// It reads a partial JSON body (UpdateBookInput), finds the existing book,
// applies only the non-nil fields from the input, and saves the changes.
// Responds 404 if the book does not exist.
func (app *applicationDependencies) updateBookHandler(w http.ResponseWriter, r *http.Request) {
	// Parse and validate the :id URL parameter.
	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Decode the partial update fields from the request body.
	var input data.UpdateBookInput
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Fetch all books and locate the one we intend to update.
	books, err := app.models.Books.GetAll()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	var book *data.Book
	for _, b := range books {
		if b.ID == id {
			book = b
			break
		}
	}

	// Return 404 if the book wasn't found.
	if book == nil {
		app.notFoundResponse(w, r)
		return
	}

	// Apply only the fields that were actually provided in the request body.
	// Each field is a pointer; nil means "not provided, leave as-is".
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

	// Persist the updated book back to the database.
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
// It parses the :id URL parameter, deletes the matching record from the database,
// and responds with a confirmation message.
// Responds 404 if no book with that ID exists.
func (app *applicationDependencies) deleteBookHandler(w http.ResponseWriter, r *http.Request) {
	// Parse and validate the :id URL parameter.
	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Delete the book from the database.
	err = app.models.Books.Delete(id)
	if err != nil {
		switch err.Error() {
		case "record not found":
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Respond with a success message.
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "book successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
