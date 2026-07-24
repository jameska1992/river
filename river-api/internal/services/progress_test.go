package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newProgressService wires a ProgressService with only the repositories a
// given test exercises; the rest are nil (unused by the methods under test).
func newProgressService(prog *memProgressRepo, eps *memEpisodeRepo) *ProgressService {
	return NewProgressService(prog, nil, eps, nil, nil, nil, nil, nil, nil)
}

func TestReport_CompletedThreshold(t *testing.T) {
	cases := []struct {
		name          string
		position      float64
		duration      float64
		wantCompleted bool
	}{
		{"past 90% is completed", 95, 100, true},
		{"exactly 90% is completed", 90, 100, true},
		{"below 90% is not completed", 50, 100, false},
		{"zero duration is not completed", 10, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memProgressRepo{}
			svc := newProgressService(repo, nil)

			p, err := svc.Report(ProgressInput{
				UserID: "u1", MediaType: "movie", MediaID: "m1",
				Position: tc.position, Duration: tc.duration,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.wantCompleted, p.Completed)

			// The row was persisted with the same completed flag.
			stored, err := repo.Find("u1", "movie", "m1")
			require.NoError(t, err)
			assert.Equal(t, tc.wantCompleted, stored.Completed)
		})
	}
}

func TestSetCompleted_FalseDeletesRow(t *testing.T) {
	repo := &memProgressRepo{}
	svc := newProgressService(repo, nil)
	_, err := svc.Report(ProgressInput{UserID: "u1", MediaType: "movie", MediaID: "m1", Position: 30, Duration: 100})
	require.NoError(t, err)

	require.NoError(t, svc.SetCompleted("u1", "movie", "m1", false))

	_, err = repo.Find("u1", "movie", "m1")
	assert.ErrorIs(t, err, ErrNotFound, "marking uncompleted should remove the progress row")
}

func TestSetCompleted_TruePreservesPosition(t *testing.T) {
	repo := &memProgressRepo{}
	svc := newProgressService(repo, nil)
	// Partway through, then explicitly mark watched.
	_, err := svc.Report(ProgressInput{UserID: "u1", MediaType: "movie", MediaID: "m1", Position: 42, Duration: 100})
	require.NoError(t, err)

	require.NoError(t, svc.SetCompleted("u1", "movie", "m1", true))

	stored, err := repo.Find("u1", "movie", "m1")
	require.NoError(t, err)
	assert.True(t, stored.Completed)
	assert.Equal(t, 42.0, stored.Position, "existing position should be preserved when marking watched")
	assert.Equal(t, 100.0, stored.Duration)
}

func TestSetCompleted_TrueWithoutExistingRow(t *testing.T) {
	repo := &memProgressRepo{}
	svc := newProgressService(repo, nil)

	require.NoError(t, svc.SetCompleted("u1", "movie", "m1", true))

	stored, err := repo.Find("u1", "movie", "m1")
	require.NoError(t, err)
	assert.True(t, stored.Completed)
	assert.Equal(t, 0.0, stored.Position)
}

func TestGetShowWatchState_CountsCompletedEpisodes(t *testing.T) {
	showID := uuid.New()
	ep1 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: showID, Number: 1}
	ep2 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: showID, Number: 2}
	ep3 := &models.Episode{Base: models.Base{ID: uuid.New()}, TVShowID: showID, Number: 3}
	eps := &memEpisodeRepo{episodes: []*models.Episode{ep1, ep2, ep3}}

	repo := &memProgressRepo{}
	svc := newProgressService(repo, eps)

	// Mark two of three episodes watched for u1.
	require.NoError(t, svc.SetCompleted("u1", "episode", ep1.ID.String(), true))
	require.NoError(t, svc.SetCompleted("u1", "episode", ep3.ID.String(), true))

	state, err := svc.GetShowWatchState("u1", showID.String())
	require.NoError(t, err)
	assert.Equal(t, 3, state.Total)
	assert.Equal(t, 2, state.Completed)
	assert.Equal(t, showID.String(), state.ShowID)
}

func TestGetShowWatchState_NoEpisodes(t *testing.T) {
	svc := newProgressService(&memProgressRepo{}, &memEpisodeRepo{})
	state, err := svc.GetShowWatchState("u1", uuid.New().String())
	require.NoError(t, err)
	assert.Equal(t, 0, state.Total)
	assert.Equal(t, 0, state.Completed)
}
