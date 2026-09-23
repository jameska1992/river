package repository

import (
	"testing"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAPITokenRepo(t *testing.T) APITokenRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.APIToken{}))
	return NewAPITokenRepository(db)
}

func TestAPITokenRepository_CreateFindRevoke(t *testing.T) {
	repo := newAPITokenRepo(t)
	owner := uuid.New()

	require.NoError(t, repo.Create(&models.APIToken{UserID: owner, Name: "grafana", TokenHash: "h1", TokenPrefix: "rvat_aaa", Scopes: `["read"]`}))

	got, err := repo.FindByHash("h1")
	require.NoError(t, err)
	assert.Equal(t, "grafana", got.Name)

	// Revoke → FindByHash (which excludes revoked) no longer returns it.
	require.NoError(t, repo.Revoke(got.ID.String()))
	_, err = repo.FindByHash("h1")
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestAPITokenRepository_ListOrderedAndTouch(t *testing.T) {
	repo := newAPITokenRepo(t)
	owner := uuid.New()
	require.NoError(t, repo.Create(&models.APIToken{UserID: owner, Name: "first", TokenHash: "h1", Scopes: "[]"}))
	require.NoError(t, repo.Create(&models.APIToken{UserID: owner, Name: "second", TokenHash: "h2", Scopes: "[]"}))

	list, err := repo.List()
	require.NoError(t, err)
	require.Len(t, list, 2)

	tok, err := repo.FindByHash("h1")
	require.NoError(t, err)
	assert.Nil(t, tok.LastUsedAt)
	require.NoError(t, repo.TouchLastUsed(tok.ID.String(), time.Now()))
	touched, err := repo.FindByHash("h1")
	require.NoError(t, err)
	assert.NotNil(t, touched.LastUsedAt)
}

func TestAPITokenRepository_RevokeUnknown(t *testing.T) {
	repo := newAPITokenRepo(t)
	assert.ErrorIs(t, repo.Revoke(uuid.NewString()), apperrors.ErrNotFound)
}
