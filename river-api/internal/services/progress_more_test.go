package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Simple passthroughs: Get / GetAllByType / Delete / (Un)DismissNextUp ---

func TestProgressService_Passthroughs(t *testing.T) {
	row := &models.WatchProgress{UserID: "u1", MediaType: "movie", MediaID: "m1", Position: 10, Duration: 100}
	prog := &memProgressRepo{rows: []*models.WatchProgress{row}}
	dismissed := &memDismissedRepo{}
	svc := NewProgressService(prog, nil, nil, nil, nil, nil, nil, nil, dismissed)

	got, err := svc.Get("u1", "movie", "m1")
	require.NoError(t, err)
	assert.Equal(t, float64(10), got.Position)

	_, err = svc.Get("u1", "movie", "missing")
	assert.Error(t, err)

	all, err := svc.GetAllByType("u1", "movie")
	require.NoError(t, err)
	assert.Len(t, all, 1)

	require.NoError(t, svc.Delete("u1", "movie", "m1"))
	_, err = svc.Get("u1", "movie", "m1")
	assert.Error(t, err, "row should be gone after Delete")

	require.NoError(t, svc.DismissNextUp("u1", "ep1"))
	assert.Equal(t, []string{"ep1"}, dismissed.episodeIDs)
	require.NoError(t, svc.UndismissNextUp("u1", "ep1"))
}

// --- ShowWatchStates ---

func TestProgressService_ShowWatchStates(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}}
	ep1 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID}
	ep2 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID}
	// Only ep1 is completed for u1.
	prog := &memProgressRepo{rows: []*models.WatchProgress{
		{UserID: "u1", MediaType: "episode", MediaID: ep1.ID.String(), Completed: true},
		{UserID: "u1", MediaType: "episode", MediaID: ep2.ID.String(), Completed: false},
	}}
	svc := NewProgressService(prog, nil,
		&memEpisodeRepo{episodes: []*models.Episode{ep1, ep2}},
		nil,
		&memShowRepo{shows: []*models.TVShow{show}},
		nil, nil, nil, nil)

	states, err := svc.ShowWatchStates("u1", "")
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.Equal(t, show.ID.String(), states[0].ShowID)
	assert.Equal(t, 2, states[0].Total)
	assert.Equal(t, 1, states[0].Completed)
}

func TestProgressService_ShowWatchStates_SkipsShowsWithNoEpisodes(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}}
	svc := NewProgressService(&memProgressRepo{}, nil,
		&memEpisodeRepo{}, nil,
		&memShowRepo{shows: []*models.TVShow{show}},
		nil, nil, nil, nil)

	states, err := svc.ShowWatchStates("u1", "")
	require.NoError(t, err)
	assert.Empty(t, states, "a show with no episodes is omitted")
}

// --- SetShowCompleted (cascade) ---

func TestProgressService_SetShowCompleted_Cascades(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}}
	ep1 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID}
	ep2 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID}
	prog := &memProgressRepo{}
	svc := NewProgressService(prog, nil,
		&memEpisodeRepo{episodes: []*models.Episode{ep1, ep2}},
		nil, &memShowRepo{shows: []*models.TVShow{show}}, nil, nil, nil, nil)

	// Mark watched → a completed row is upserted for every episode.
	require.NoError(t, svc.SetShowCompleted("u1", show.ID.String(), true))
	assert.Len(t, prog.rows, 2)
	for _, r := range prog.rows {
		assert.True(t, r.Completed)
	}

	// Mark unwatched → every episode row is deleted.
	require.NoError(t, svc.SetShowCompleted("u1", show.ID.String(), false))
	assert.Empty(t, prog.rows)
}

// --- ActiveSessions ---

func TestProgressService_ActiveSessions_ResolvesTitles(t *testing.T) {
	user := &models.User{Base: models.Base{ID: uuid.New()}, Username: "alice"}
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Nosferatu"}
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Dragnet"}
	ep := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, Number: 3, Title: "The Big Case"}
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Dune"}
	ch := &models.AudiobookChapter{Base: models.Base{ID: uuid.New()}, AudiobookID: book.ID, Number: 1} // no title → "Chapter 1"

	prog := &memProgressRepo{active: []models.WatchProgress{
		{UserID: user.ID.String(), MediaType: "movie", MediaID: movie.ID.String()},
		{UserID: user.ID.String(), MediaType: "episode", MediaID: ep.ID.String()},
		{UserID: user.ID.String(), MediaType: "chapter", MediaID: ch.ID.String()},
		{UserID: "ghost", MediaType: "movie", MediaID: movie.ID.String()}, // unknown user → skipped
	}}
	svc := NewProgressService(prog,
		&memMovieRepo{movies: []*models.Movie{movie}},
		&memEpisodeRepo{episodes: []*models.Episode{ep}},
		nil,
		&memShowRepo{shows: []*models.TVShow{show}},
		&memUserRepo{users: []*models.User{user}},
		&memAudiobookRepo{books: []*models.Audiobook{book}},
		&memChapterRepo{chapters: []*models.AudiobookChapter{ch}},
		nil)

	items, err := svc.ActiveSessions()
	require.NoError(t, err)
	require.Len(t, items, 3, "the session for the unknown user is dropped")

	assert.Equal(t, "Nosferatu", items[0].Title)
	assert.Equal(t, "alice", items[0].Username)

	assert.Equal(t, "The Big Case", items[1].Title)
	assert.Equal(t, "Dragnet", items[1].ShowTitle)

	assert.Equal(t, "Chapter 1", items[2].Title, "missing chapter title falls back to number")
	assert.Equal(t, "Dune", items[2].ShowTitle)
}

// --- GetUserActivity ---

func TestProgressService_GetUserActivity_ResolvesTitles(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis"}
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Firefly"}
	ep := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, Number: 2} // no title → "Episode 2"
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "1984"}
	ch := &models.AudiobookChapter{Base: models.Base{ID: uuid.New()}, AudiobookID: book.ID, Number: 4, Title: "Room 101"}

	prog := &memProgressRepo{byUser: []models.WatchProgress{
		{UserID: "u1", MediaType: "movie", MediaID: movie.ID.String()},
		{UserID: "u1", MediaType: "episode", MediaID: ep.ID.String()},
		{UserID: "u1", MediaType: "chapter", MediaID: ch.ID.String()},
		{UserID: "u1", MediaType: "podcast", MediaID: "unknown"}, // default branch
	}}
	svc := NewProgressService(prog,
		&memMovieRepo{movies: []*models.Movie{movie}},
		&memEpisodeRepo{episodes: []*models.Episode{ep}},
		nil,
		&memShowRepo{shows: []*models.TVShow{show}},
		nil,
		&memAudiobookRepo{books: []*models.Audiobook{book}},
		&memChapterRepo{chapters: []*models.AudiobookChapter{ch}},
		nil)

	items, err := svc.GetUserActivity("u1")
	require.NoError(t, err)
	require.Len(t, items, 4)

	assert.Equal(t, "Metropolis", items[0].Title)
	assert.Equal(t, "Episode 2", items[1].Title)
	assert.Equal(t, "Firefly", items[1].ShowTitle)
	assert.Equal(t, "Room 101", items[2].Title)
	assert.Equal(t, "1984", items[2].ShowTitle)
	assert.Equal(t, "podcast", items[3].Title, "unknown media type falls back to the type name")
}

// --- NextUp cross-season rollover (exercises pickFirstInSeason) ---

func TestProgressService_NextUp_RollsIntoNextSeason(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Show"}
	s1 := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, Number: 1}
	s2 := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, Number: 2}
	s1e1 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, SeasonID: s1.ID, Number: 1}
	s2e1 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, SeasonID: s2.ID, Number: 1, Title: "Season 2 Premiere"}

	// Anchor: the last (only) episode of season 1 is completed.
	prog := &memProgressRepo{completedEpisodes: []models.WatchProgress{
		{UserID: "u1", MediaType: "episode", MediaID: s1e1.ID.String()},
	}}
	svc := NewProgressService(prog,
		&memMovieRepo{},
		&memEpisodeRepo{episodes: []*models.Episode{s1e1, s2e1}},
		&memSeasonRepo{seasons: []*models.Season{s1, s2}},
		&memShowRepo{shows: []*models.TVShow{show}},
		nil, &memAudiobookRepo{}, &memChapterRepo{}, &memDismissedRepo{})

	items, err := svc.NextUp("u1", 16)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, s2e1.ID.String(), items[0].MediaID)
	assert.Equal(t, 2, items[0].SeasonNumber)
	assert.Equal(t, "Season 2 Premiere", items[0].Title)
}

// --- NextEpisode ---

func newNextEpisodeService(prog *memProgressRepo, seasons *memSeasonRepo, eps *memEpisodeRepo) *ProgressService {
	return NewProgressService(prog, nil, eps, seasons, nil, nil, nil, nil, nil)
}

func TestProgressService_NextEpisode_FirstIncomplete(t *testing.T) {
	showID := uuid.New()
	season := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: showID, Number: 1}
	ep1 := &models.Episode{Base: models.Base{ID: uuid.New()}, SeasonID: season.ID, Number: 1, FilePath: "/1.mp4"}
	ep2 := &models.Episode{Base: models.Base{ID: uuid.New()}, SeasonID: season.ID, Number: 2, FilePath: "/2.mp4"}
	// ep1 completed, ep2 not → NextEpisode should return ep2.
	prog := &memProgressRepo{rows: []*models.WatchProgress{
		{UserID: "u1", MediaType: "episode", MediaID: ep1.ID.String(), Completed: true},
	}}
	svc := newNextEpisodeService(prog,
		&memSeasonRepo{seasons: []*models.Season{season}},
		&memEpisodeRepo{episodes: []*models.Episode{ep1, ep2}})

	res, err := svc.NextEpisode("u1", showID.String())
	require.NoError(t, err)
	assert.Equal(t, ep2.ID.String(), res.EpisodeID)
	assert.Equal(t, season.ID.String(), res.SeasonID)
}

func TestProgressService_NextEpisode_AllCompletedRestartsAtFirst(t *testing.T) {
	showID := uuid.New()
	season := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: showID, Number: 1}
	ep1 := &models.Episode{Base: models.Base{ID: uuid.New()}, SeasonID: season.ID, Number: 1, FilePath: "/1.mp4"}
	ep2 := &models.Episode{Base: models.Base{ID: uuid.New()}, SeasonID: season.ID, Number: 2, FilePath: "/2.mp4"}
	prog := &memProgressRepo{rows: []*models.WatchProgress{
		{UserID: "u1", MediaType: "episode", MediaID: ep1.ID.String(), Completed: true},
		{UserID: "u1", MediaType: "episode", MediaID: ep2.ID.String(), Completed: true},
	}}
	svc := newNextEpisodeService(prog,
		&memSeasonRepo{seasons: []*models.Season{season}},
		&memEpisodeRepo{episodes: []*models.Episode{ep1, ep2}})

	res, err := svc.NextEpisode("u1", showID.String())
	require.NoError(t, err)
	assert.Equal(t, ep1.ID.String(), res.EpisodeID, "everything watched → restart at the first episode")
}

func TestProgressService_NextEpisode_NoPlayableEpisodes(t *testing.T) {
	showID := uuid.New()
	season := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: showID, Number: 1}
	// Episode with no FilePath is skipped → nothing playable → ErrNotFound.
	ep := &models.Episode{Base: models.Base{ID: uuid.New()}, SeasonID: season.ID, Number: 1}
	svc := newNextEpisodeService(&memProgressRepo{},
		&memSeasonRepo{seasons: []*models.Season{season}},
		&memEpisodeRepo{episodes: []*models.Episode{ep}})

	_, err := svc.NextEpisode("u1", showID.String())
	assert.Error(t, err)
}
