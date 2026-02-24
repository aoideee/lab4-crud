// cmd/api/routes.go
package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// routes registers all HTTP endpoints and returns the configured router as an http.Handler.
// The router is wired to the applicationDependencies receiver so every handler
// has access to the logger, config, and database models.
//
// Current endpoints:
//
//	POST   /v1/books        – create a new book
//	GET    /v1/books/:id    – retrieve a single book by ID
//	GET    /v1/books        – list all books
//	PATCH  /v1/books/:id    – partially update an existing book
//	DELETE /v1/books/:id    – delete a book by ID
func (app *applicationDependencies) routes() http.Handler {
	router := httprouter.New()

	// Book CRUD routes
	router.HandlerFunc(http.MethodPost,   "/v1/books",     app.createBookHandler)
	router.HandlerFunc(http.MethodGet,    "/v1/books/:id", app.showBookHandler)
	router.HandlerFunc(http.MethodGet,    "/v1/books",     app.listBooksHandler)
	router.HandlerFunc(http.MethodPatch,  "/v1/books/:id", app.updateBookHandler)
	router.HandlerFunc(http.MethodDelete, "/v1/books/:id", app.deleteBookHandler)

	return router
}