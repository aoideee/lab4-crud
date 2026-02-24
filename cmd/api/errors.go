// cmd/api/errors.go
// This file contains all error-response helpers for the application.
// Keeping error helpers in a dedicated file makes them easy to find and extend.
package main

import (
	"log/slog"
	"net/http"
)

// logError logs an internal error at ERROR level with the request method and URL for context.
func (app *applicationDependencies) logError(r *http.Request, err error) {
	app.logger.Error(err.Error(),
		slog.String("request_method", r.Method),
		slog.String("request_url", r.URL.String()),
	)
}

// errorResponse sends a JSON error envelope with the given status code and message.
// It is the low-level building block used by all the specific error helpers below.
func (app *applicationDependencies) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	data := envelope{"error": message}
	err := app.writeJSON(w, status, data, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// serverErrorResponse logs a 500-level error and sends a generic message to the client.
// We never expose internal error details to the client for security reasons.
func (app *applicationDependencies) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)
	app.errorResponse(w, r, http.StatusInternalServerError, "the server encountered a problem and could not process your request")
}

// notFoundResponse sends a 404 Not Found error.
func (app *applicationDependencies) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	app.errorResponse(w, r, http.StatusNotFound, "the requested resource could not be found")
}

// methodNotAllowedResponse sends a 405 Method Not Allowed error.
func (app *applicationDependencies) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := "the " + r.Method + " method is not supported for this resource"
	app.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

// badRequestResponse sends a 400 Bad Request error with the error message from the caller.
func (app *applicationDependencies) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// failedValidationResponse sends a 422 Unprocessable Entity response containing
// the field-level validation errors collected by a Validator.
func (app *applicationDependencies) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

// rateLimitExceededResponse sends a 429 Too Many Requests error.
func (app *applicationDependencies) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request) {
	app.errorResponse(w, r, http.StatusTooManyRequests, "rate limit exceeded")
}
