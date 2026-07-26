package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortClause(t *testing.T) {
	whitelist := map[string]string{
		"title": "LOWER(title)",
		"year":  "year",
	}
	fallback := "created_at"

	cases := []struct {
		name         string
		field, order string
		want         string
	}{
		{"known field asc", "year", "asc", "year ASC"},
		{"known field desc", "year", "desc", "year DESC"},
		{"whitelist expression preserved", "title", "asc", "LOWER(title) ASC"},
		{"unknown field falls back", "bogus", "asc", "created_at ASC"},
		{"case-insensitive field", "YEAR", "asc", "year ASC"},
		{"trimmed field", "  year  ", "asc", "year ASC"},
		{"case-insensitive desc", "year", "DESC", "year DESC"},
		{"trimmed order", "year", "  desc  ", "year DESC"},
		{"unknown order defaults asc", "year", "sideways", "year ASC"},
		{"empty order defaults asc", "year", "", "year ASC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sortClause(tc.field, tc.order, whitelist, fallback))
		})
	}
}

func TestTitleSortExpr(t *testing.T) {
	expr := titleSortExpr("title")
	// The exact SQL is an implementation detail, but it must reference the
	// column and lowercase for case-insensitive ordering.
	assert.Contains(t, expr, "title")
	assert.True(t, strings.Contains(expr, "LOWER"), "expression should lowercase for case-insensitive sort")
}
