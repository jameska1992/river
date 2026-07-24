package services

import (
	"errors"
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMovieService_Create_DefaultsGenres(t *testing.T) {
	svc := NewMovieService(&memMovieRepo{}, &memCleanupRepo{})
	m, err := svc.Create(MovieInput{Title: "Nosferatu"})
	require.NoError(t, err)
	assert.Equal(t, "[]", m.Genres, "empty genres should default to a JSON empty array")
}

func TestMovieService_Update_TMDBIDIsSticky(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Old", TMDBID: 555, Genres: `["horror"]`}
	repo := &memMovieRepo{movies: []*models.Movie{movie}}
	svc := NewMovieService(repo, &memCleanupRepo{})

	// Generic edit sends TMDBID 0 — the resolved id must be preserved.
	updated, err := svc.Update(movie.ID.String(), MovieInput{Title: "New", TMDBID: 0})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Title)
	assert.Equal(t, 555, updated.TMDBID, "TMDBID 0 must not erase the resolved id")

	// A real enrichment id (>0) does overwrite.
	updated, err = svc.Update(movie.ID.String(), MovieInput{Title: "New", TMDBID: 777})
	require.NoError(t, err)
	assert.Equal(t, 777, updated.TMDBID)
}

func TestMovieService_Update_PreservesBlankFields(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Genres: `["horror"]`, FilePath: "/v.mp4", SourcePath: "/src.mkv"}
	repo := &memMovieRepo{movies: []*models.Movie{movie}}
	svc := NewMovieService(repo, &memCleanupRepo{})

	// Blank Genres/FilePath/SourcePath in the input must not wipe existing values.
	updated, err := svc.Update(movie.ID.String(), MovieInput{Title: "T"})
	require.NoError(t, err)
	assert.Equal(t, `["horror"]`, updated.Genres)
	assert.Equal(t, "/v.mp4", updated.FilePath)
	assert.Equal(t, "/src.mkv", updated.SourcePath)
}

func TestMovieService_ClearTMDBID(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, TMDBID: 42}
	repo := &memMovieRepo{movies: []*models.Movie{movie}}
	svc := NewMovieService(repo, &memCleanupRepo{})

	require.NoError(t, svc.ClearTMDBID(movie.ID.String()))
	got, err := repo.FindByID(movie.ID.String())
	require.NoError(t, err)
	assert.Equal(t, 0, got.TMDBID)
}

func TestMovieService_Delete_PurgesThenDeletes(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Gone"}
	repo := &memMovieRepo{movies: []*models.Movie{movie}}
	cleanup := &memCleanupRepo{}
	svc := NewMovieService(repo, cleanup)

	require.NoError(t, svc.Delete(movie.ID.String()))
	assert.Contains(t, cleanup.purged, "movie:"+movie.ID.String(), "cross-references should be purged")
	_, err := repo.FindByID(movie.ID.String())
	assert.ErrorIs(t, err, ErrNotFound, "the movie row should be deleted")
}

func TestMovieService_Delete_PurgeErrorAborts(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Stays"}
	repo := &memMovieRepo{movies: []*models.Movie{movie}}
	cleanup := &memCleanupRepo{movieErr: errors.New("purge failed")}
	svc := NewMovieService(repo, cleanup)

	err := svc.Delete(movie.ID.String())
	assert.Error(t, err, "a purge failure should propagate")
	_, findErr := repo.FindByID(movie.ID.String())
	assert.NoError(t, findErr, "the row must NOT be deleted when purge fails")
}

func TestMovieService_Similar_NoGenresIsEmpty(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Genres: "[]"}
	repo := &memMovieRepo{movies: []*models.Movie{movie}}
	svc := NewMovieService(repo, &memCleanupRepo{})

	got, err := svc.Similar(movie.ID.String(), 10)
	require.NoError(t, err)
	assert.Empty(t, got, "a movie with no genres has no similar titles")
}
