package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nextUpFixture builds a one-season show with two episodes and returns a
// ProgressService plus the ids, with episode 1 marked completed (the
// anchor). Callers tweak the progress/dismissed fakes for each case.
type nextUpFixture struct {
	svc       *ProgressService
	prog      *memProgressRepo
	dismissed *memDismissedRepo
	showID    uuid.UUID
	ep1, ep2  *models.Episode
}

func newNextUpFixture() nextUpFixture {
	showID := uuid.New()
	seasonID := uuid.New()
	show := &models.TVShow{Base: models.Base{ID: showID}, Title: "Dragnet", PosterPath: "/p.jpg"}
	season := &models.Season{Base: models.Base{ID: seasonID}, TVShowID: showID, Number: 1}
	ep1 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: showID, SeasonID: seasonID, Number: 1, Title: "Pilot"}
	ep2 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: showID, SeasonID: seasonID, Number: 2, Title: "The Big Case"}

	prog := &memProgressRepo{
		completedEpisodes: []models.WatchProgress{{UserID: "u1", MediaType: "episode", MediaID: ep1.ID.String()}},
	}
	dismissed := &memDismissedRepo{}
	svc := NewProgressService(prog,
		&memMovieRepo{},
		&memEpisodeRepo{episodes: []*models.Episode{ep1, ep2}},
		&memSeasonRepo{seasons: []*models.Season{season}},
		&memShowRepo{shows: []*models.TVShow{show}},
		nil, &memAudiobookRepo{}, &memChapterRepo{}, dismissed)

	return nextUpFixture{svc: svc, prog: prog, dismissed: dismissed, showID: showID, ep1: ep1, ep2: ep2}
}

func TestNextUp_ReturnsNextEpisodeInSeason(t *testing.T) {
	f := newNextUpFixture()

	items, err := f.svc.NextUp("u1", 16)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, f.ep2.ID.String(), items[0].MediaID)
	assert.Equal(t, "The Big Case", items[0].Title)
	assert.Equal(t, "Dragnet", items[0].ShowTitle)
	assert.Equal(t, 1, items[0].SeasonNumber)
	assert.Equal(t, 2, items[0].EpisodeNumber)
}

func TestNextUp_ExcludesAlreadyStartedNext(t *testing.T) {
	f := newNextUpFixture()
	// The next episode already has a progress row => it belongs on
	// Continue Watching, not Next Up.
	f.prog.rows = []*models.WatchProgress{{UserID: "u1", MediaType: "episode", MediaID: f.ep2.ID.String()}}

	items, err := f.svc.NextUp("u1", 16)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestNextUp_ExcludesDismissed(t *testing.T) {
	f := newNextUpFixture()
	f.dismissed.episodeIDs = []string{f.ep2.ID.String()}

	items, err := f.svc.NextUp("u1", 16)
	require.NoError(t, err)
	assert.Empty(t, items, "a dismissed next episode should not resurface")
}

func TestNextUp_NoNextAfterLastEpisode(t *testing.T) {
	f := newNextUpFixture()
	// Anchor on the LAST episode — there's nothing after it.
	f.prog.completedEpisodes = []models.WatchProgress{{UserID: "u1", MediaType: "episode", MediaID: f.ep2.ID.String()}}

	items, err := f.svc.NextUp("u1", 16)
	require.NoError(t, err)
	assert.Empty(t, items)
}
