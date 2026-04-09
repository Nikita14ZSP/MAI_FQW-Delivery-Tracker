package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common domain error conditions.
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
)

// Wrap wraps an error with an additional message while preserving the original
// error for use with errors.Is and errors.As.
func Wrap(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}
