package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by the service layer. Callers (REST, MCP, CLI)
// map them to transport-specific responses.
var (
	// ErrNotFound is returned when a requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalid is returned when a request payload fails validation.
	ErrInvalid = errors.New("invalid request")
	// ErrConflict is returned when an operation conflicts with current state
	// (e.g. completing an issue without a comment, or a status transition that
	// is not allowed).
	ErrConflict = errors.New("conflict")
	// ErrProjectRequired is returned when no project is specified.
	ErrProjectRequired = errors.New("project is required")
)

// NewInvalid wraps err into an ErrInvalid with a cause message.
func NewInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
