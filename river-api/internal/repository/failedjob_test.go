package repository

import (
	"testing"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newFailedJobRepo(t *testing.T) FailedJobRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.FailedJob{}))
	return NewFailedJobRepository(db)
}

func TestFailedJobRepository_CreateListFilterDelete(t *testing.T) {
	repo := newFailedJobRepo(t)

	require.NoError(t, repo.Create(&models.FailedJob{Service: "river-video-trans", MediaType: "movie", Reason: "boom", Attempts: 5}))
	require.NoError(t, repo.Create(&models.FailedJob{Service: "river-meta-tv", MediaType: "tvshow", Reason: "429", Attempts: 3}))

	all, total, err := repo.List(ListFailedJobsFilter{Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, all, 2)

	// Filter by service.
	vids, total, err := repo.List(ListFailedJobsFilter{Service: "river-video-trans", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, vids, 1)
	assert.Equal(t, "movie", vids[0].MediaType)

	// Delete one, then it's gone; deleting again is ErrNotFound.
	id := vids[0].ID.String()
	require.NoError(t, repo.Delete(id))
	_, err = repo.FindByID(id)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
	assert.ErrorIs(t, repo.Delete(id), apperrors.ErrNotFound)

	_, total, err = repo.List(ListFailedJobsFilter{Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}
