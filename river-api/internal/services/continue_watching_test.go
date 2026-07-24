package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func progressServiceForCW(prog *memProgressRepo, movies *memMovieRepo, eps *memEpisodeRepo, seasons *memSeasonRepo, shows *memShowRepo, books *memAudiobookRepo, chapters *memChapterRepo) *ProgressService {
	// users + dismissedNext repos are unused by ContinueWatching.
	return NewProgressService(prog, movies, eps, seasons, shows, nil, books, chapters, nil)
}

func TestContinueWatching_Movie(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis", PosterPath: "/p.jpg", BackdropPath: "/b.jpg"}
	prog := &memProgressRepo{inProgress: []models.WatchProgress{
		{UserID: "u1", MediaType: "movie", MediaID: movie.ID.String(), Position: 30, Duration: 100},
	}}
	svc := progressServiceForCW(prog, &memMovieRepo{movies: []*models.Movie{movie}}, &memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{}, &memAudiobookRepo{}, &memChapterRepo{})

	items, err := svc.ContinueWatching("u1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Metropolis", items[0].Title)
	assert.Equal(t, "/p.jpg", items[0].PosterPath)
	assert.Equal(t, "/b.jpg", items[0].BackdropPath)
}

func TestContinueWatching_Episode(t *testing.T) {
	showID := uuid.New()
	seasonID := uuid.New()
	ep := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: showID, SeasonID: seasonID, Number: 3, Title: "The Big Case"}
	season := &models.Season{Base: models.Base{ID: seasonID}, TVShowID: showID, Number: 2}
	show := &models.TVShow{Base: models.Base{ID: showID}, Title: "Dragnet", PosterPath: "/sp.jpg", BackdropPath: "/sb.jpg"}
	prog := &memProgressRepo{inProgress: []models.WatchProgress{
		{UserID: "u1", MediaType: "episode", MediaID: ep.ID.String(), Position: 10, Duration: 50},
	}}
	svc := progressServiceForCW(prog, &memMovieRepo{},
		&memEpisodeRepo{episodes: []*models.Episode{ep}},
		&memSeasonRepo{seasons: []*models.Season{season}},
		&memShowRepo{shows: []*models.TVShow{show}},
		&memAudiobookRepo{}, &memChapterRepo{})

	items, err := svc.ContinueWatching("u1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	it := items[0]
	assert.Equal(t, "The Big Case", it.Title)
	assert.Equal(t, "Dragnet", it.ShowTitle)
	assert.Equal(t, showID.String(), it.ShowID)
	assert.Equal(t, 2, it.SeasonNumber)
	assert.Equal(t, 3, it.EpisodeNumber)
}

func TestContinueWatching_DeduplicatesAudiobookChapters(t *testing.T) {
	bookID := uuid.New()
	book := &models.Audiobook{Base: models.Base{ID: bookID}, Title: "Dracula", CoverPath: "/c.jpg"}
	ch1 := &models.AudiobookChapter{Base: models.Base{ID: uuid.New()}, AudiobookID: bookID, Number: 1}
	ch2 := &models.AudiobookChapter{Base: models.Base{ID: uuid.New()}, AudiobookID: bookID, Number: 2}
	// Two chapters of the same book are both "in progress"; only the first
	// (most-recent) should surface so one book can't dominate the rail.
	prog := &memProgressRepo{inProgress: []models.WatchProgress{
		{UserID: "u1", MediaType: "chapter", MediaID: ch2.ID.String()},
		{UserID: "u1", MediaType: "chapter", MediaID: ch1.ID.String()},
	}}
	svc := progressServiceForCW(prog, &memMovieRepo{}, &memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{},
		&memAudiobookRepo{books: []*models.Audiobook{book}},
		&memChapterRepo{chapters: []*models.AudiobookChapter{ch1, ch2}})

	items, err := svc.ContinueWatching("u1")
	require.NoError(t, err)
	require.Len(t, items, 1, "a single audiobook should appear once regardless of chapters touched")
	assert.Equal(t, "Dracula", items[0].Title)
	assert.Equal(t, bookID.String(), items[0].AudiobookID)
}

func TestContinueWatching_SkipsStaleReferences(t *testing.T) {
	// Progress points at a movie that no longer exists — it should be
	// silently dropped rather than erroring the whole rail.
	prog := &memProgressRepo{inProgress: []models.WatchProgress{
		{UserID: "u1", MediaType: "movie", MediaID: uuid.New().String()},
	}}
	svc := progressServiceForCW(prog, &memMovieRepo{}, &memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{}, &memAudiobookRepo{}, &memChapterRepo{})

	items, err := svc.ContinueWatching("u1")
	require.NoError(t, err)
	assert.Empty(t, items)
}
