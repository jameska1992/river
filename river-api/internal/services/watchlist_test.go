package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchlistService_AddEnrichesEntry(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Nosferatu", Year: 1922, PosterPath: "/n.jpg"}
	svc := NewWatchlistService(&memWatchlistRepo{}, &memMovieRepo{movies: []*models.Movie{movie}}, &memShowRepo{}, &memAudiobookRepo{})

	entry, err := svc.Add("u1", "movie", movie.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Nosferatu", entry.Title)
	assert.Equal(t, 1922, entry.Year)
	assert.Equal(t, "/n.jpg", entry.PosterPath)
	assert.NotEmpty(t, entry.ID)
}

func TestWatchlistService_AddIsIdempotent(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Nosferatu"}
	repo := &memWatchlistRepo{}
	svc := NewWatchlistService(repo, &memMovieRepo{movies: []*models.Movie{movie}}, &memShowRepo{}, &memAudiobookRepo{})

	first, err := svc.Add("u1", "movie", movie.ID.String())
	require.NoError(t, err)
	second, err := svc.Add("u1", "movie", movie.ID.String())
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID, "re-adding the same media should return the existing entry")
	list, err := svc.List("u1")
	require.NoError(t, err)
	assert.Len(t, list, 1, "no duplicate row should be created")
}

func TestWatchlistService_ListEnrichesAcrossTypes(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Nosferatu"}
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "The Lone Ranger"}
	svc := NewWatchlistService(
		&memWatchlistRepo{},
		&memMovieRepo{movies: []*models.Movie{movie}},
		&memShowRepo{shows: []*models.TVShow{show}},
		&memAudiobookRepo{},
	)
	_, err := svc.Add("u1", "movie", movie.ID.String())
	require.NoError(t, err)
	_, err = svc.Add("u1", "tvshow", show.ID.String())
	require.NoError(t, err)

	list, err := svc.List("u1")
	require.NoError(t, err)
	require.Len(t, list, 2)

	titles := map[string]string{}
	for _, e := range list {
		titles[e.MediaType] = e.Title
	}
	assert.Equal(t, "Nosferatu", titles["movie"])
	assert.Equal(t, "The Lone Ranger", titles["tvshow"])
}

func TestWatchlistService_ListMissingMediaLeavesTitleBlank(t *testing.T) {
	repo := &memWatchlistRepo{}
	svc := NewWatchlistService(repo, &memMovieRepo{}, &memShowRepo{}, &memAudiobookRepo{})
	// Add a watchlist row that references a movie which no longer exists.
	_, err := svc.Add("u1", "movie", uuid.New().String())
	require.NoError(t, err)

	list, err := svc.List("u1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Empty(t, list[0].Title, "a dangling media reference should enrich to a blank title, not error")
}

func TestWatchlistService_Remove(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Nosferatu"}
	svc := NewWatchlistService(&memWatchlistRepo{}, &memMovieRepo{movies: []*models.Movie{movie}}, &memShowRepo{}, &memAudiobookRepo{})
	entry, err := svc.Add("u1", "movie", movie.ID.String())
	require.NoError(t, err)

	require.NoError(t, svc.Remove("u1", entry.ID))
	list, err := svc.List("u1")
	require.NoError(t, err)
	assert.Empty(t, list)
}
