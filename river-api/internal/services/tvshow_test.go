package services

import (
	"errors"
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTVShowService wires a TVShowService; seasons is nil since the methods
// under test here don't touch the season repository.
func newTVShowService(shows *memShowRepo, episodes *memEpisodeRepo, cleanup *memCleanupRepo) *TVShowService {
	return NewTVShowService(shows, nil, episodes, cleanup)
}

func TestTVShowService_CreateShow_DefaultsGenres(t *testing.T) {
	svc := newTVShowService(&memShowRepo{}, &memEpisodeRepo{}, &memCleanupRepo{})
	show, err := svc.CreateShow(TVShowInput{Title: "Dragnet"})
	require.NoError(t, err)
	assert.Equal(t, "[]", show.Genres)
}

func TestTVShowService_UpdateShow_TMDBIDIsSticky(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Old", TMDBID: 100, Genres: `["western"]`}
	repo := &memShowRepo{shows: []*models.TVShow{show}}
	svc := newTVShowService(repo, &memEpisodeRepo{}, &memCleanupRepo{})

	updated, err := svc.UpdateShow(show.ID.String(), TVShowInput{Title: "New", TMDBID: 0})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Title)
	assert.Equal(t, 100, updated.TMDBID, "TMDBID 0 must not erase the resolved id")

	updated, err = svc.UpdateShow(show.ID.String(), TVShowInput{Title: "New", TMDBID: 200})
	require.NoError(t, err)
	assert.Equal(t, 200, updated.TMDBID)
}

func TestTVShowService_UpdateShow_PreservesBlankFields(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Genres: `["western"]`, FolderPath: "/tv/show"}
	repo := &memShowRepo{shows: []*models.TVShow{show}}
	svc := newTVShowService(repo, &memEpisodeRepo{}, &memCleanupRepo{})

	updated, err := svc.UpdateShow(show.ID.String(), TVShowInput{Title: "T"})
	require.NoError(t, err)
	assert.Equal(t, `["western"]`, updated.Genres)
	assert.Equal(t, "/tv/show", updated.FolderPath)
}

func TestTVShowService_ClearTMDBID(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, TMDBID: 9}
	repo := &memShowRepo{shows: []*models.TVShow{show}}
	svc := newTVShowService(repo, &memEpisodeRepo{}, &memCleanupRepo{})

	require.NoError(t, svc.ClearTMDBID(show.ID.String()))
	got, err := repo.FindByID(show.ID.String())
	require.NoError(t, err)
	assert.Equal(t, 0, got.TMDBID)
}

func TestTVShowService_DeleteShow_PurgesThenDeletes(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Gone"}
	repo := &memShowRepo{shows: []*models.TVShow{show}}
	cleanup := &memCleanupRepo{}
	svc := newTVShowService(repo, &memEpisodeRepo{}, cleanup)

	require.NoError(t, svc.DeleteShow(show.ID.String()))
	assert.Contains(t, cleanup.purged, "show:"+show.ID.String())
	_, err := repo.FindByID(show.ID.String())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestTVShowService_DeleteShow_PurgeErrorAborts(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Stays"}
	repo := &memShowRepo{shows: []*models.TVShow{show}}
	svc := newTVShowService(repo, &memEpisodeRepo{}, &memCleanupRepo{showErr: errors.New("purge failed")})

	err := svc.DeleteShow(show.ID.String())
	assert.Error(t, err)
	_, findErr := repo.FindByID(show.ID.String())
	assert.NoError(t, findErr, "the show must not be deleted when purge fails")
}

func TestTVShowService_DeleteEpisode_PurgesThenDeletes(t *testing.T) {
	ep := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: uuid.New(), Number: 1}
	eps := &memEpisodeRepo{episodes: []*models.Episode{ep}}
	cleanup := &memCleanupRepo{}
	svc := newTVShowService(&memShowRepo{}, eps, cleanup)

	require.NoError(t, svc.DeleteEpisode(ep.ID.String()))
	assert.Contains(t, cleanup.purged, "episode:"+ep.ID.String())
	_, err := eps.FindByID(ep.ID.String())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestTVShowService_DeleteEpisode_PurgeErrorAborts(t *testing.T) {
	ep := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: uuid.New(), Number: 1}
	eps := &memEpisodeRepo{episodes: []*models.Episode{ep}}
	svc := newTVShowService(&memShowRepo{}, eps, &memCleanupRepo{episodeErr: errors.New("purge failed")})

	err := svc.DeleteEpisode(ep.ID.String())
	assert.Error(t, err)
	_, findErr := eps.FindByID(ep.ID.String())
	assert.NoError(t, findErr, "the episode must not be deleted when purge fails")
}
