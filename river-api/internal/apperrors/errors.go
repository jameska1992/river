package apperrors

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidInput signals a caller-supplied value failed validation
	// (enum/range check). Handlers map it to 400; wrap it with a message
	// describing what was invalid.
	ErrInvalidInput = errors.New("invalid input")
)
