package services

import "river-api/internal/apperrors"

// Re-export so callers (handlers) need only import services.
var (
	ErrNotFound     = apperrors.ErrNotFound
	ErrConflict     = apperrors.ErrConflict
	ErrUnauthorized = apperrors.ErrUnauthorized
)
