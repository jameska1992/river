package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover the race where an admin merges a TV show while one of its
// episodes is still transcoding: river-video-trans finishes holding the
// pre-merge show/season ids, and the late CreateEpisode/GetShow must redirect
// onto the survivor so the transcoded file still registers instead of being
// orphaned.

// Case 1: the merged season was re-parented wholesale onto the survivor (same
// season id, new tv_show_id). A create carrying the merged show id resolves the
// survivor season and attaches the episode there.
func TestCreateEpisode_MergedShow_MovedSeason_RedirectsToSurvivor(t *testing.T) {
	survivorID, mergedID, seasonID := uuid.New(), uuid.New(), uuid.New()
	seasons := &memSeasonRepo{seasons: []*models.Season{
		{Base: models.Base{ID: seasonID}, TVShowID: survivorID, Number: 2},
	}}
	episodes := &memEpisodeRepo{}
	merge := &fakeMergeRepo{survivor: map[string]string{mergedID.String(): survivorID.String()}}
	svc := NewTVShowService(&memShowRepo{}, seasons, episodes, &memCleanupRepo{}, merge)

	ep, err := svc.CreateEpisode(mergedID.String(), seasonID.String(), EpisodeInput{
		Number: 3, FilePath: "/out/s2e3.mp4", SourcePath: "/src/s2e3.mkv",
	})
	require.NoError(t, err)
	assert.Equal(t, seasonID, ep.SeasonID)
	assert.Equal(t, survivorID, ep.TVShowID, "episode must attach to the survivor show")
	assert.Equal(t, "/out/s2e3.mp4", ep.FilePath)
	require.Len(t, episodes.episodes, 1, "exactly one episode row created")
}

// Case 2: the merged season collided with a same-numbered survivor season and
// was soft-deleted, its episodes folded in. A create carrying the (now deleted)
// merged season id is redirected by number and backfills the survivor's
// existing episode rather than creating a duplicate.
func TestCreateEpisode_MergedShow_SameNumberSeason_BackfillsSurvivorEpisode(t *testing.T) {
	survivorID, mergedID := uuid.New(), uuid.New()
	survivorSeasonID, mergedSeasonID := uuid.New(), uuid.New()
	// Survivor already carries season 1 with episode 3 (created by meta-tv),
	// but with no file yet.
	existingEp := &models.Episode{
		Base: models.Base{ID: uuid.New()}, TVShowID: survivorID, SeasonID: survivorSeasonID, Number: 3,
	}
	seasons := &memSeasonRepo{
		seasons: []*models.Season{{Base: models.Base{ID: survivorSeasonID}, TVShowID: survivorID, Number: 1}},
		deleted: []*models.Season{{Base: models.Base{ID: mergedSeasonID}, TVShowID: mergedID, Number: 1}},
	}
	episodes := &memEpisodeRepo{episodes: []*models.Episode{existingEp}}
	merge := &fakeMergeRepo{survivor: map[string]string{mergedID.String(): survivorID.String()}}
	svc := NewTVShowService(&memShowRepo{}, seasons, episodes, &memCleanupRepo{}, merge)

	ep, err := svc.CreateEpisode(mergedID.String(), mergedSeasonID.String(), EpisodeInput{
		Number: 3, FilePath: "/out/s1e3.mp4",
	})
	require.NoError(t, err)
	assert.Equal(t, existingEp.ID, ep.ID, "should backfill the survivor's episode, not duplicate")
	assert.Equal(t, "/out/s1e3.mp4", ep.FilePath)
	require.Len(t, episodes.episodes, 1, "no duplicate episode created")
}

// A genuine unknown season (no merge alias) must still surface ErrNotFound —
// the redirect must not mask real errors.
func TestCreateEpisode_UnknownSeason_NoMergeAlias_StillNotFound(t *testing.T) {
	merge := &fakeMergeRepo{}
	svc := NewTVShowService(&memShowRepo{}, &memSeasonRepo{}, &memEpisodeRepo{}, &memCleanupRepo{}, merge)

	_, err := svc.CreateEpisode(uuid.NewString(), uuid.NewString(), EpisodeInput{Number: 1})
	assert.ErrorIs(t, err, ErrNotFound)
}

// GetShow follows the merge redirect so a caller holding a pre-merge show id
// (e.g. river-video-trans fetching the title before writing back) resolves the
// survivor rather than 404-ing and dropping the whole event.
func TestGetShow_MergedAway_RedirectsToSurvivor(t *testing.T) {
	survivor := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Survivor"}
	mergedID := uuid.New()
	shows := &memShowRepo{shows: []*models.TVShow{survivor}}
	merge := &fakeMergeRepo{survivor: map[string]string{mergedID.String(): survivor.ID.String()}}
	svc := NewTVShowService(shows, &memSeasonRepo{}, &memEpisodeRepo{}, &memCleanupRepo{}, merge)

	got, err := svc.GetShow(mergedID.String())
	require.NoError(t, err)
	assert.Equal(t, survivor.ID, got.ID)
}

// A plain-deleted / never-existed show with no alias stays a not-found.
func TestGetShow_UnknownNoAlias_NotFound(t *testing.T) {
	merge := &fakeMergeRepo{}
	svc := NewTVShowService(&memShowRepo{}, &memSeasonRepo{}, &memEpisodeRepo{}, &memCleanupRepo{}, merge)

	_, err := svc.GetShow(uuid.NewString())
	assert.ErrorIs(t, err, ErrNotFound)
}

// A chain of merges (A folded into B, then B into C) resolves to the terminal
// survivor without spinning.
func TestGetShow_ChainedMerge_ResolvesTerminalSurvivor(t *testing.T) {
	c := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "C"}
	aID, bID := uuid.New(), uuid.New()
	shows := &memShowRepo{shows: []*models.TVShow{c}}
	merge := &fakeMergeRepo{survivor: map[string]string{
		aID.String(): bID.String(),
		bID.String(): c.ID.String(),
	}}
	svc := NewTVShowService(shows, &memSeasonRepo{}, &memEpisodeRepo{}, &memCleanupRepo{}, merge)

	got, err := svc.GetShow(aID.String())
	require.NoError(t, err)
	assert.Equal(t, c.ID, got.ID)
}
