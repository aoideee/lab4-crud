// cmd/api/helpers.go
// This file contains general-purpose helper functions for the application.
// Error-response helpers live in errors.go; only non-error utilities are here.
package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

// envelope is the top-level JSON wrapper type used for all API responses.
// Every response body is a JSON object with at least one named key,
// e.g. {"book": {...}} or {"books": [...], "metadata": {...}}.
type envelope map[string]any

// readIDParam extracts and validates the ":id" URL parameter added by httprouter.
// Returns an error if the value is missing, non-numeric, or less than 1.
func (app *applicationDependencies) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

// readString reads a string query parameter from qs, returning defaultValue
// if the key is absent or empty.
func (app *applicationDependencies) readString(qs url.Values, key, defaultValue string) string {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	return s
}

// readInt reads an integer query parameter from qs, returning defaultValue if
// the key is absent or cannot be parsed as an integer.
func (app *applicationDependencies) readInt(qs url.Values, key string, defaultValue int) int {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return i
}

// writeJSON marshals data to indented JSON, applies any custom headers,
// sets Content-Type to "application/json", writes the status code, and
// streams the body to the client.
func (app *applicationDependencies) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	js = append(js, '\n') // Trailing newline makes curl output nicer.

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

// readJSON decodes a single JSON value from the request body into dst.
// It enforces a 1 MB size limit, rejects unknown fields, and ensures the
// body contains exactly one JSON value (no trailing data).
func (app *applicationDependencies) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// Cap the request body to 1 MB to prevent large-payload attacks.
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // Reject fields not present in dst.

	err := dec.Decode(dst)
	if err != nil {
		return err
	}

	// Ensure there is no second JSON value in the body.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}