package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// capturePool is a gorm.ConnPool that records the SQL GORM builds and then
// short-circuits with an error, so the repository query methods can be exercised
// (and their generated SQL asserted) without a live Postgres.
type capturePool struct{ queries []string }

func (p *capturePool) PrepareContext(ctx context.Context, q string) (*sql.Stmt, error) {
	return nil, sql.ErrConnDone
}
func (p *capturePool) ExecContext(ctx context.Context, q string, a ...any) (sql.Result, error) {
	p.queries = append(p.queries, q)
	return nil, sql.ErrConnDone
}
func (p *capturePool) QueryContext(ctx context.Context, q string, a ...any) (*sql.Rows, error) {
	p.queries = append(p.queries, q)
	return nil, sql.ErrConnDone
}
func (p *capturePool) QueryRowContext(ctx context.Context, q string, a ...any) *sql.Row {
	return nil
}

func (p *capturePool) last() string {
	if len(p.queries) == 0 {
		return ""
	}
	return p.queries[len(p.queries)-1]
}

func newCaptureRepo(t *testing.T) (SearchRepository, *capturePool) {
	t.Helper()
	pool := &capturePool{}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewSearchRepository(db), pool
}

func TestSearchMovies_RanksByRelevance(t *testing.T) {
	repo, pool := newCaptureRepo(t)
	_, _ = repo.SearchMovies("x", "", 50)

	sql := pool.last()
	// The exact-match tier is what pushes the film "X" ahead of the many
	// titles that merely contain an "x".
	assert.Contains(t, sql, "CASE WHEN LOWER(title) =")
	assert.Contains(t, sql, "similarity(LOWER(title),")
	assert.Contains(t, sql, `FROM "movies"`)
}

func TestSearchMovies_GenreFilterAndFuzzyMatch(t *testing.T) {
	repo, pool := newCaptureRepo(t)
	_, _ = repo.SearchMovies("x", "Action", 50)

	sql := pool.last()
	assert.Contains(t, sql, "LOWER(title) LIKE")
	assert.Contains(t, sql, "genres LIKE")
}

func TestSearchMovies_EmptyQueryFallsBackToAlphabetical(t *testing.T) {
	repo, pool := newCaptureRepo(t)
	_, _ = repo.SearchMovies("", "Action", 50)

	sql := pool.last()
	assert.Contains(t, sql, "ORDER BY title")
	assert.NotContains(t, sql, "similarity", "genre-only browse should not rank by similarity")
}

func TestSearchTVShows_RanksByRelevance(t *testing.T) {
	repo, pool := newCaptureRepo(t)
	_, _ = repo.SearchTVShows("x", "", 50)

	sql := pool.last()
	assert.Contains(t, sql, "CASE WHEN LOWER(title) =")
	assert.Contains(t, sql, `FROM "tv_shows"`)
}

func TestSearchAudiobooks_MatchesTitleAndAuthor(t *testing.T) {
	repo, pool := newCaptureRepo(t)
	_, _ = repo.SearchAudiobooks("asimov", "", 50)

	sql := pool.last()
	assert.Contains(t, sql, "LOWER(title) LIKE")
	assert.Contains(t, sql, "LOWER(author) LIKE")
	assert.Contains(t, sql, "similarity(LOWER(author),")
}

func TestSearchAudiobooks_EmptyQueryFallsBackToAlphabetical(t *testing.T) {
	repo, pool := newCaptureRepo(t)
	_, _ = repo.SearchAudiobooks("", "sci-fi", 50)

	sql := pool.last()
	assert.Contains(t, sql, "ORDER BY title")
	assert.Contains(t, sql, "LOWER(genre) LIKE")
	assert.NotContains(t, sql, "similarity")
}

func TestSearchPeople_RanksByRelevance(t *testing.T) {
	repo, pool := newCaptureRepo(t)
	_, _ = repo.SearchPeople("bela", 15)

	sql := pool.last()
	assert.Contains(t, sql, "CASE WHEN LOWER(name) =")
	assert.Contains(t, sql, "similarity(LOWER(name),")
	assert.Contains(t, sql, `FROM "people"`)
	assert.True(t, strings.Contains(sql, "LIMIT"), "limit should be applied")
}
