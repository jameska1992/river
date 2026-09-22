package repository

import (
	"testing"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newServiceKeyRepo(t *testing.T) ServiceKeyRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ServiceKey{}))
	return NewServiceKeyRepository(db)
}

func TestServiceKeyRepository_CreateFindRevoke(t *testing.T) {
	repo := newServiceKeyRepo(t)

	require.NoError(t, repo.Create(&models.ServiceKey{Name: "river-scan", KeyHash: "hash1", KeyPrefix: "rvk_aaa", Scopes: `["media:write"]`}))

	got, err := repo.FindByHash("hash1")
	require.NoError(t, err)
	assert.Equal(t, "river-scan", got.Name)

	byName, err := repo.FindByName("river-scan")
	require.NoError(t, err)
	assert.Equal(t, "hash1", byName.KeyHash)

	// Revoke → no longer found by hash (FindByHash excludes revoked).
	require.NoError(t, repo.Revoke(got.ID.String()))
	_, err = repo.FindByHash("hash1")
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestServiceKeyRepository_CreateIfAbsent(t *testing.T) {
	repo := newServiceKeyRepo(t)

	created, err := repo.CreateIfAbsent(&models.ServiceKey{Name: "river-meta-tv", KeyHash: "h1", Scopes: "[]"})
	require.NoError(t, err)
	assert.True(t, created)

	// Same name, different hash → not created, original preserved.
	created, err = repo.CreateIfAbsent(&models.ServiceKey{Name: "river-meta-tv", KeyHash: "h2", Scopes: "[]"})
	require.NoError(t, err)
	assert.False(t, created)

	got, err := repo.FindByName("river-meta-tv")
	require.NoError(t, err)
	assert.Equal(t, "h1", got.KeyHash, "existing key never clobbered")
}

func TestServiceKeyRepository_DuplicateNameRejected(t *testing.T) {
	repo := newServiceKeyRepo(t)
	require.NoError(t, repo.Create(&models.ServiceKey{Name: "dup", KeyHash: "a", Scopes: "[]"}))
	assert.Error(t, repo.Create(&models.ServiceKey{Name: "dup", KeyHash: "b", Scopes: "[]"}))
}

func TestServiceKeyRepository_ListAndTouch(t *testing.T) {
	repo := newServiceKeyRepo(t)
	require.NoError(t, repo.Create(&models.ServiceKey{Name: "b-svc", KeyHash: "hb", Scopes: "[]"}))
	require.NoError(t, repo.Create(&models.ServiceKey{Name: "a-svc", KeyHash: "ha", Scopes: "[]"}))

	list, err := repo.List()
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "a-svc", list[0].Name, "ordered by name")

	k, _ := repo.FindByName("a-svc")
	require.NoError(t, repo.TouchLastUsed(k.ID.String(), time.Now()))
	k2, _ := repo.FindByName("a-svc")
	assert.NotNil(t, k2.LastUsedAt)
}

func TestServiceKeyRepository_RevokeUnknown(t *testing.T) {
	repo := newServiceKeyRepo(t)
	assert.ErrorIs(t, repo.Revoke("00000000-0000-0000-0000-000000000000"), apperrors.ErrNotFound)
}
