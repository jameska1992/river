package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

// parsePaginationQuery extracts page and limit from query params.
func parsePaginationQuery(c *gin.Context) (page, limit int) {
	page = 1
	limit = 50
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	return
}

// parseSimilarLimit reads the ?limit=N query on /*/similar endpoints.
// Defaults to 16 (matches the client-side row cap) and clamps to a max
// of 50 so a poorly-behaved client can't ask us to sort the entire
// library. Non-numeric / non-positive values fall back to the default.
func parseSimilarLimit(raw string) int {
	if raw == "" {
		return 16
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 16
	}
	if n > 50 {
		return 50
	}
	return n
}

// serviceStatus maps service-layer sentinel errors to HTTP status codes.
func serviceStatus(err error) int {
	switch {
	case errors.Is(err, services.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, services.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, services.ErrUnauthorized):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
