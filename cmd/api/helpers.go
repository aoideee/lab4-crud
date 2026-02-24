// cmd/api/helpers.go

package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

// Define an envelope type for structured JSON responses
type envelope map[string]any

// readIDParam parses the "id" parameter from the URL
func (app *applicationDependencies) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

// writeJSON sends a JSON response with an envelope and custom headers
func (app *applicationDependencies) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

// readJSON decodes the request body into a target Go variable
func (app *applicationDependencies) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	maxBytes := 1_048_576 // 1MB limit to prevent DOS attacks
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // Reject JSON that doesn't match our struct

	err := dec.Decode(dst)
	if err != nil {
		return err // In a real app, you'd handle specific syntax errors here
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

// badRequestResponse is a quick helper for sending 400 errors
func (app *applicationDependencies) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.writeJSON(w, http.StatusBadRequest, envelope{"error": err.Error()}, nil)
}

// serverErrorResponse handles 500 errors and logs them
func (app *applicationDependencies) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error(err.Error())
	app.writeJSON(w, http.StatusInternalServerError, envelope{"error": "the server encountered a problem"}, nil)
}

// notFoundResponse sends a 404 error
func (app *applicationDependencies) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusNotFound, envelope{"error": "the requested resource could not be found"}, nil)
}