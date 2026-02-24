// cmd/api/routes.go
package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// routes registers all HTTP endpoints and returns the configured router wrapped
// in the recoverPanic and rateLimit middlewares.
//
// Middleware chain (outermost → innermost):
//
//	recoverPanic → rateLimit → router
//
// Current endpoints:
//
//	POST   /v1/books        – create a new book
//	GET    /v1/books/:id    – retrieve a single book by ID
//	GET    /v1/books        – list all books (paginated)
//	PATCH  /v1/books/:id    – partially update an existing book
//	DELETE /v1/books/:id    – delete a book by ID
func (app *applicationDependencies) routes() http.Handler {
	router := httprouter.New()

	// Override the default httprouter error handlers to return JSON responses.
	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	// Book CRUD routes
	router.HandlerFunc(http.MethodPost,   "/v1/books",     app.createBookHandler)
	router.HandlerFunc(http.MethodGet,    "/v1/books/:id", app.showBookHandler)
	router.HandlerFunc(http.MethodGet,    "/v1/books",     app.listBooksHandler)
	router.HandlerFunc(http.MethodPatch,  "/v1/books/:id", app.updateBookHandler)
	router.HandlerFunc(http.MethodDelete, "/v1/books/:id", app.deleteBookHandler)

	// Wrap with middleware: recoverPanic is outermost so it catches panics
	// from rateLimit and router alike.
	return app.recoverPanic(app.rateLimit(router))
}