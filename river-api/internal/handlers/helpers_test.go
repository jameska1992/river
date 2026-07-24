package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestServiceStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", services.ErrNotFound, http.StatusNotFound},
		{"conflict", services.ErrConflict, http.StatusConflict},
		{"unauthorized", services.ErrUnauthorized, http.StatusUnauthorized},
		{"wrapped not found", fmt.Errorf("lookup: %w", services.ErrNotFound), http.StatusNotFound},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, serviceStatus(tc.err))
		})
	}
}

func ctxWithQuery(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	return c
}

func TestParsePaginationQuery(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantPage  int
		wantLimit int
	}{
		{"defaults when absent", "", 1, 50},
		{"explicit values", "page=3&limit=20", 3, 20},
		{"non-numeric falls back", "page=abc&limit=xyz", 1, 50},
		{"non-positive falls back", "page=0&limit=-5", 1, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, limit := parsePaginationQuery(ctxWithQuery(tc.query))
			assert.Equal(t, tc.wantPage, page)
			assert.Equal(t, tc.wantLimit, limit)
		})
	}
}

func TestParseSimilarLimit(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"", 16},
		{"abc", 16},
		{"0", 16},
		{"-5", 16},
		{"10", 10},
		{"50", 50},
		{"100", 50}, // clamped to the max
	}
	for _, tc := range cases {
		t.Run("raw="+tc.raw, func(t *testing.T) {
			assert.Equal(t, tc.want, parseSimilarLimit(tc.raw))
		})
	}
}
