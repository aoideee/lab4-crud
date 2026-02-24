// Package validator provides a custom Validator type for accumulating
// field-level validation errors and returning them as a map.
package validator

import "regexp"

// EmailRX is a compiled regular expression for basic email validation.
var EmailRX = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// Validator holds a map of field names to their validation error messages.
// A Validator with an empty Errors map is considered valid.
type Validator struct {
	Errors map[string]string
}

// New creates and returns a fresh, empty Validator.
func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

// Valid returns true if the Errors map contains no entries.
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// AddError records key as failing with the given message.
// If key already has an error it is not overwritten, so the first
// failure for a field is always the one that is reported.
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

// Check adds an error for key with message only when ok is false.
// Use this as a single-line guard:
//
//	v.Check(len(title) > 0, "title", "must be provided")
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// In returns true if value is present in the list slice.
func In(value string, list ...string) bool {
	for _, item := range list {
		if value == item {
			return true
		}
	}
	return false
}

// Matches returns true if value matches the provided compiled regexp.
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// Unique returns true if every string in values is distinct.
func Unique(values []string) bool {
	seen := make(map[string]bool)
	for _, v := range values {
		if seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
