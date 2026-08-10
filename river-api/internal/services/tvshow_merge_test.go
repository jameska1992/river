package services

import (
	"testing"
	"time"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMergeRepo records the Merge call so tests can assert orientation and the
// alias passed down, without a real DB transaction.
type fakeMergeRepo struct {
	called               bool
	survivorID, mergedID string
	alias                *models.TVShowPath
	err                  error
	// survivor maps a merged-away show id to the id of the show that absorbed
	// it, backing FindSurvivorID.
	survivor map[string]string
}

func (f *fakeMergeRepo) Merge(survivorID, mergedID string, alias *models.TVShowPath) error {
	f.called = true
	f.survivorID, f.mergedID, f.alias = survivorID, mergedID, alias
	return f.err
}

func (f *fakeMergeRepo) FindShowIDByPath(string, string) (string, error) { return "", ErrNotFound }

func (f *fakeMergeRepo) FindSurvivorID(mergedID string) (string, error) {
	if id, ok := f.survivor[mergedID]; ok {
		return id, nil
	}
	return "", ErrNotFound
}

func mgShow(id uuid.UUID, created time.Time, folder string) *models.TVShow {
	return &models.TVShow{
		Base:       models.Base{ID: id, CreatedAt: created},
		Title:      "Show",
		FolderPath: folder,
		LibraryID:  uuid.New(),
	}
}
func mgSeason(id, showID uuid.UUID, num int) *models.Season {
	return &models.Season{Base: models.Base{ID: id}, TVShowID: showID, Number: num}
}
func mgEp(id, seasonID, showID uuid.UUID, num int) *models.Episode {
	return &models.Episode{Base: models.Base{ID: id}, SeasonID: seasonID, TVShowID: showID, Number: num}
}

var mgEpoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func TestShowMerge_OldestSurvivesAndAbsorbedRootPreserved(t *testing.T) {
	older, newer := uuid.New(), uuid.New()
	shows := &memShowRepo{shows: []*models.TVShow{
		mgShow(older, mgEpoch, "/media/shows/MyShow"),
		mgShow(newer, mgEpoch.Add(time.Hour), "/truenas/tv/MyShow"),
	}}
	// Disjoint seasons (S1 on older, S2 on newer) → no episode collisions.
	s1, s2 := uuid.New(), uuid.New()
	seasons := &memSeasonRepo{seasons: []*models.Season{mgSeason(s1, older, 1), mgSeason(s2, newer, 2)}}
	eps := &memEpisodeRepo{episodes: []*models.Episode{
		mgEp(uuid.New(), s1, older, 1),
		mgEp(uuid.New(), s2, newer, 1),
	}}
	merge := &fakeMergeRepo{}
	svc := NewShowMergeService(shows, seasons, eps, merge)

	// Pass newer-first to prove orientation ignores argument order.
	got, err := svc.Merge(newer.String(), older.String())
	require.NoError(t, err)
	assert.Equal(t, older, got.ID, "the older row survives")

	require.True(t, merge.called)
	assert.Equal(t, older.String(), merge.survivorID)
	assert.Equal(t, newer.String(), merge.mergedID)
	require.NotNil(t, merge.alias, "absorbed folder must be preserved as an alias")
	assert.Equal(t, "/truenas/tv/MyShow", merge.alias.FolderPath)
	assert.Equal(t, older, merge.alias.TVShowID)
}

func TestShowMerge_PreviewCountsDisjointSeasons(t *testing.T) {
	older, newer := uuid.New(), uuid.New()
	shows := &memShowRepo{shows: []*models.TVShow{
		mgShow(older, mgEpoch, "/a/MyShow"),
		mgShow(newer, mgEpoch.Add(time.Hour), "/b/MyShow"),
	}}
	s1, s2 := uuid.New(), uuid.New()
	seasons := &memSeasonRepo{seasons: []*models.Season{mgSeason(s1, older, 1), mgSeason(s2, newer, 2)}}
	eps := &memEpisodeRepo{episodes: []*models.Episode{
		mgEp(uuid.New(), s2, newer, 1),
		mgEp(uuid.New(), s2, newer, 2),
	}}
	svc := NewShowMergeService(shows, seasons, eps, &fakeMergeRepo{})

	prev, err := svc.PreviewMerge(older.String(), newer.String())
	require.NoError(t, err)
	assert.Equal(t, older, prev.Survivor.ID)
	assert.Equal(t, newer, prev.Merged.ID)
	assert.Equal(t, 1, prev.SeasonsMoved)
	assert.Equal(t, 2, prev.EpisodesMoved)
	assert.Empty(t, prev.Conflicts)
	assert.True(t, prev.CanMerge)
}

func TestShowMerge_CollidingEpisodesBlockMerge(t *testing.T) {
	older, newer := uuid.New(), uuid.New()
	shows := &memShowRepo{shows: []*models.TVShow{
		mgShow(older, mgEpoch, "/a/MyShow"),
		mgShow(newer, mgEpoch.Add(time.Hour), "/b/MyShow"),
	}}
	// Both have a Season 1 with an episode 1 → collision.
	s1o, s1n := uuid.New(), uuid.New()
	seasons := &memSeasonRepo{seasons: []*models.Season{mgSeason(s1o, older, 1), mgSeason(s1n, newer, 1)}}
	eps := &memEpisodeRepo{episodes: []*models.Episode{
		mgEp(uuid.New(), s1o, older, 1),
		mgEp(uuid.New(), s1n, newer, 1),
	}}
	merge := &fakeMergeRepo{}
	svc := NewShowMergeService(shows, seasons, eps, merge)

	prev, err := svc.PreviewMerge(older.String(), newer.String())
	require.NoError(t, err)
	require.Len(t, prev.Conflicts, 1)
	assert.Equal(t, 1, prev.Conflicts[0].SeasonNumber)
	assert.Equal(t, 1, prev.Conflicts[0].EpisodeNumber)
	assert.False(t, prev.CanMerge)

	_, err = svc.Merge(older.String(), newer.String())
	assert.ErrorIs(t, err, ErrConflict)
	assert.False(t, merge.called, "a conflicting merge must not touch the repo")
}

func TestShowMerge_SameShowRejected(t *testing.T) {
	id := uuid.New()
	shows := &memShowRepo{shows: []*models.TVShow{mgShow(id, mgEpoch, "/a/MyShow")}}
	svc := NewShowMergeService(shows, &memSeasonRepo{}, &memEpisodeRepo{}, &fakeMergeRepo{})

	_, err := svc.Merge(id.String(), id.String())
	assert.ErrorIs(t, err, ErrConflict)
}

func TestShowMerge_UnknownShowIsNotFound(t *testing.T) {
	known := uuid.New()
	shows := &memShowRepo{shows: []*models.TVShow{mgShow(known, mgEpoch, "/a/MyShow")}}
	svc := NewShowMergeService(shows, &memSeasonRepo{}, &memEpisodeRepo{}, &fakeMergeRepo{})

	_, err := svc.PreviewMerge(known.String(), uuid.New().String())
	assert.ErrorIs(t, err, ErrNotFound)
}
