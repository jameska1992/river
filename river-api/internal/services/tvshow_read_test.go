package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTVShowService_ShowReadPassthroughs(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Firefly", Genres: `["sci-fi"]`}
	shows := &memShowRepo{shows: []*models.TVShow{show}}
	svc := NewTVShowService(shows, &memSeasonRepo{}, &memEpisodeRepo{}, &memCleanupRepo{})

	list, err := svc.ListShows(TVShowFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	got, err := svc.GetShow(show.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Firefly", got.Title)

	_, err = svc.GetShow(uuid.NewString())
	assert.Error(t, err)

	_, err = svc.CountShows("")
	require.NoError(t, err)
	_, err = svc.ListRecentShows(5)
	require.NoError(t, err)
	_, err = svc.ListUnidentifiedShows()
	require.NoError(t, err)

	updated, err := svc.UpdateFolderPath(show.ID.String(), "/tv/Firefly")
	require.NoError(t, err)
	assert.Equal(t, "/tv/Firefly", updated.FolderPath)
	assert.Equal(t, "Firefly", updated.Title, "title must be untouched")

	_, err = svc.UpdateFolderPath(uuid.NewString(), "/x")
	assert.Error(t, err)
}

func TestTVShowService_SimilarShows_RanksByGenre(t *testing.T) {
	src := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Src", Genres: `["Crime","Drama"]`}
	match := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Match", Genres: `["crime"]`}
	miss := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Miss", Genres: `["comedy"]`}
	shows := &memShowRepo{shows: []*models.TVShow{src, match, miss}}
	svc := NewTVShowService(shows, &memSeasonRepo{}, &memEpisodeRepo{}, &memCleanupRepo{})

	got, err := svc.SimilarShows(src.ID.String(), 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Match", got[0].Title)
	assert.Equal(t, "tvshow", got[0].Type)
}

func TestTVShowService_Seasons(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Show"}
	season := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, Number: 1, Title: "S1"}
	shows := &memShowRepo{shows: []*models.TVShow{show}}
	seasons := &memSeasonRepo{seasons: []*models.Season{season}}
	svc := NewTVShowService(shows, seasons, &memEpisodeRepo{}, &memCleanupRepo{})

	list, err := svc.ListSeasons(show.ID.String())
	require.NoError(t, err)
	assert.Len(t, list, 1)

	created, err := svc.CreateSeason(show.ID.String(), SeasonInput{Number: 2, Title: "S2"})
	require.NoError(t, err)
	assert.Equal(t, show.ID, created.TVShowID)
	assert.Equal(t, 2, created.Number)

	_, err = svc.CreateSeason(uuid.NewString(), SeasonInput{Number: 1})
	assert.Error(t, err, "unknown show should error")

	// UpdateSeason has PATCH semantics — blank/zero fields are skipped.
	upd, err := svc.UpdateSeason(show.ID.String(), season.ID.String(), SeasonInput{Title: "Renamed"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", upd.Title)
	assert.Equal(t, 1, upd.Number, "zero Number must not clobber existing")

	_, err = svc.UpdateSeason(show.ID.String(), uuid.NewString(), SeasonInput{Title: "x"})
	assert.Error(t, err)
}

func TestTVShowService_Episodes(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Show"}
	season := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, Number: 1}
	shows := &memShowRepo{shows: []*models.TVShow{show}}
	seasons := &memSeasonRepo{seasons: []*models.Season{season}}
	episodes := &memEpisodeRepo{}
	svc := NewTVShowService(shows, seasons, episodes, &memCleanupRepo{})

	// Create (no existing → new row).
	ep, err := svc.CreateEpisode(show.ID.String(), season.ID.String(), EpisodeInput{
		Number: 1, Title: "Pilot", FilePath: "/s1e1.mp4", AiredAt: "2002-09-20T00:00:00Z",
	})
	require.NoError(t, err)
	assert.Equal(t, "Pilot", ep.Title)
	assert.False(t, ep.AiredAt.IsZero(), "valid RFC3339 AiredAt should parse")

	// Create with the same season+number → update path (updateEpisodeFields).
	again, err := svc.CreateEpisode(show.ID.String(), season.ID.String(), EpisodeInput{
		Number: 1, Title: "Pilot (remastered)",
	})
	require.NoError(t, err)
	assert.Equal(t, "Pilot (remastered)", again.Title)

	list, err := svc.ListEpisodes(season.ID.String())
	require.NoError(t, err)
	assert.Len(t, list, 1)

	got, err := svc.GetEpisode(ep.ID.String())
	require.NoError(t, err)
	assert.Equal(t, ep.ID, got.ID)

	updated, err := svc.UpdateEpisode(ep.ID.String(), EpisodeInput{Description: "desc", Runtime: 42})
	require.NoError(t, err)
	assert.Equal(t, "desc", updated.Description)
	assert.Equal(t, 42, updated.Runtime)

	srcUpd, err := svc.UpdateEpisodeSourcePath(ep.ID.String(), "/src/s1e1.mkv")
	require.NoError(t, err)
	assert.Equal(t, "/src/s1e1.mkv", srcUpd.SourcePath)

	_, err = svc.UpdateEpisode(uuid.NewString(), EpisodeInput{Title: "x"})
	assert.Error(t, err)
	_, err = svc.CreateEpisode(show.ID.String(), uuid.NewString(), EpisodeInput{Number: 1})
	assert.Error(t, err, "unknown season should error")
}

func TestTVShowService_UpdateEpisode_Reparent(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Show"}
	s1 := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, Number: 1}
	s2 := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, Number: 2}
	ep := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: show.ID, SeasonID: s1.ID, Number: 3}
	shows := &memShowRepo{shows: []*models.TVShow{show}}
	seasons := &memSeasonRepo{seasons: []*models.Season{s1, s2}}
	episodes := &memEpisodeRepo{episodes: []*models.Episode{ep}}
	svc := NewTVShowService(shows, seasons, episodes, &memCleanupRepo{})

	moved, err := svc.UpdateEpisode(ep.ID.String(), EpisodeInput{SeasonID: s2.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, s2.ID, moved.SeasonID, "episode should be reparented to the new season")

	// Reparenting onto an unknown season must fail rather than orphan the row.
	_, err = svc.UpdateEpisode(ep.ID.String(), EpisodeInput{SeasonID: uuid.NewString()})
	assert.Error(t, err)
}
