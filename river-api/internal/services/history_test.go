package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory_IncludesCompletedAndDedupesAudiobook(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis", PosterPath: "/p.jpg"}
	bookID := uuid.New()
	book := &models.Audiobook{Base: models.Base{ID: bookID}, Title: "Dracula", CoverPath: "/c.jpg"}
	ch1 := &models.AudiobookChapter{Base: models.Base{ID: uuid.New()}, AudiobookID: bookID, Number: 2}
	ch2 := &models.AudiobookChapter{Base: models.Base{ID: uuid.New()}, AudiobookID: bookID, Number: 1}

	// FindByUser returns rows most-recent-first: a completed movie, then two
	// chapters of the same book.
	prog := &memProgressRepo{byUser: []models.WatchProgress{
		{UserID: "u1", MediaType: "movie", MediaID: movie.ID.String(), Position: 100, Duration: 100, Completed: true},
		{UserID: "u1", MediaType: "chapter", MediaID: ch1.ID.String(), Position: 20, Duration: 60},
		{UserID: "u1", MediaType: "chapter", MediaID: ch2.ID.String(), Position: 5, Duration: 60},
	}}
	svc := progressServiceForCW(prog, &memMovieRepo{movies: []*models.Movie{movie}},
		&memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{},
		&memAudiobookRepo{books: []*models.Audiobook{book}},
		&memChapterRepo{chapters: []*models.AudiobookChapter{ch1, ch2}})

	items, err := svc.History("u1")
	require.NoError(t, err)
	require.Len(t, items, 2, "completed movie kept, audiobook collapsed to one row")

	assert.Equal(t, "Metropolis", items[0].Title)
	assert.True(t, items[0].Completed, "history must include completed items")

	assert.Equal(t, "Dracula", items[1].Title)
	assert.Equal(t, bookID.String(), items[1].AudiobookID)
	assert.Equal(t, 2, items[1].ChapterNumber, "first (most-recent) chapter row is the one kept")
}

func TestHistory_DropsStaleReferences(t *testing.T) {
	// A progress row pointing at a since-deleted movie is dropped, not errored.
	prog := &memProgressRepo{byUser: []models.WatchProgress{
		{UserID: "u1", MediaType: "movie", MediaID: uuid.NewString()},
	}}
	svc := progressServiceForCW(prog, &memMovieRepo{}, &memEpisodeRepo{}, &memSeasonRepo{},
		&memShowRepo{}, &memAudiobookRepo{}, &memChapterRepo{})

	items, err := svc.History("u1")
	require.NoError(t, err)
	assert.Empty(t, items)
}
