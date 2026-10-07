// Package domain holds the core entities, business rules and the repository
// and usecase contracts of the HRMS. It depends only on the standard library.
package domain

import (
	"errors"
	"sort"
	"strings"
)

// Sentinel errors returned by repositories and usecases. The HTTP layer maps
// them to status codes, so callers should wrap them with %w rather than
// replacing them.
var (
	ErrNotFound                = errors.New("resource not found")
	ErrConflict                = errors.New("resource already exists")
	ErrInvalidInput            = errors.New("invalid input")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrForbidden               = errors.New("forbidden")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	// ErrRuleViolation means the input is well-formed but a business rule
	// forbids the operation, e.g. promoting an employee with a recent penalty.
	ErrRuleViolation = errors.New("business rule violation")
)

// ValidationError collects per-field validation messages.
// errors.Is(err, ErrInvalidInput) reports true for any *ValidationError.
type ValidationError struct {
	Fields map[string]string
}

func NewValidationError() *ValidationError {
	return &ValidationError{Fields: make(map[string]string)}
}

// Add records a message for field. The first message for a field wins.
func (e *ValidationError) Add(field, message string) {
	if _, exists := e.Fields[field]; !exists {
		e.Fields[field] = message
	}
}

func (e *ValidationError) HasErrors() bool {
	return len(e.Fields) > 0
}

// Err returns e as an error when it has fields, or a nil error otherwise.
func (e *ValidationError) Err() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}

func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+e.Fields[k])
	}
	return ErrInvalidInput.Error() + ": " + strings.Join(parts, "; ")
}

func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}
