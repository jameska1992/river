package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuzzyMatch(t *testing.T) {
	where, args := fuzzyMatch("LOWER(title)", "matrix")
	assert.Equal(t, "LOWER(title) LIKE ? OR similarity(LOWER(title), ?) > 0.3", where)
	assert.Equal(t, []any{"%matrix%", "matrix"}, args)
}

func TestFuzzyMatchAny(t *testing.T) {
	where, args := fuzzyMatchAny([]string{"LOWER(title)", "LOWER(author)"}, "asimov")
	assert.Equal(t,
		"(LOWER(title) LIKE ? OR similarity(LOWER(title), ?) > 0.3) OR "+
			"(LOWER(author) LIKE ? OR similarity(LOWER(author), ?) > 0.3)",
		where)
	assert.Equal(t, []any{"%asimov%", "asimov", "%asimov%", "asimov"}, args)
}

func TestRelevanceOrder(t *testing.T) {
	order, args := relevanceOrder("LOWER(title)", "title", "x")

	// Tiers, in priority order: exact, prefix, word-start, substring.
	assert.Contains(t, order, "WHEN LOWER(title) = ? THEN 0")
	assert.Contains(t, order, "WHEN LOWER(title) LIKE ? THEN 1")
	assert.Contains(t, order, "WHEN LOWER(title) LIKE ? THEN 2")
	assert.Contains(t, order, "WHEN LOWER(title) LIKE ? THEN 3")
	// Ties broken by similarity, then the raw column alphabetically.
	assert.Contains(t, order, "similarity(LOWER(title), ?) DESC, title")

	// Args line up with the tiers: exact, prefix, word-start, substring, sim.
	// This is the crux of the "X" fix: an exact match binds "x" to the = ?
	// arm and lands in bucket 0 ahead of every incidental substring match.
	assert.Equal(t, []any{"x", "x%", "% x%", "%x%", "x"}, args)
}
