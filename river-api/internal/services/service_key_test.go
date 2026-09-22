package services

import (
	"testing"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memServiceKeyRepo is an in-memory ServiceKeyRepository fake.
type memServiceKeyRepo struct {
	keys []*models.ServiceKey
}

func (m *memServiceKeyRepo) Create(k *models.ServiceKey) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	for _, e := range m.keys {
		if e.Name == k.Name {
			return apperrors.ErrConflict
		}
	}
	m.keys = append(m.keys, k)
	return nil
}
func (m *memServiceKeyRepo) CreateIfAbsent(k *models.ServiceKey) (bool, error) {
	for _, e := range m.keys {
		if e.Name == k.Name {
			return false, nil
		}
	}
	return true, m.Create(k)
}
func (m *memServiceKeyRepo) FindByHash(hash string) (*models.ServiceKey, error) {
	for _, e := range m.keys {
		if e.KeyHash == hash && !e.Revoked {
			return e, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memServiceKeyRepo) FindByName(name string) (*models.ServiceKey, error) {
	for _, e := range m.keys {
		if e.Name == name {
			return e, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memServiceKeyRepo) List() ([]models.ServiceKey, error) {
	out := make([]models.ServiceKey, len(m.keys))
	for i, e := range m.keys {
		out[i] = *e
	}
	return out, nil
}
func (m *memServiceKeyRepo) Revoke(id string) error {
	for _, e := range m.keys {
		if e.ID.String() == id {
			e.Revoked = true
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (m *memServiceKeyRepo) TouchLastUsed(id string, at time.Time) error {
	for _, e := range m.keys {
		if e.ID.String() == id {
			e.LastUsedAt = &at
			return nil
		}
	}
	return apperrors.ErrNotFound
}

func TestServiceKey_MintAndAuthenticate(t *testing.T) {
	svc := NewServiceKeyService(&memServiceKeyRepo{})

	plaintext, view, err := svc.Mint("river-meta-movie", []string{ScopeSettingsTMDB, ScopeMediaWrite})
	require.NoError(t, err)
	assert.True(t, len(plaintext) > len(ServiceKeyPrefix))
	assert.Contains(t, plaintext, ServiceKeyPrefix)
	assert.Equal(t, "river-meta-movie", view.Name)
	assert.ElementsMatch(t, []string{ScopeSettingsTMDB, ScopeMediaWrite}, view.Scopes)

	// The minted plaintext authenticates and returns its name + scopes.
	name, scopes, ok := svc.AuthenticateServiceKey(plaintext)
	assert.True(t, ok)
	assert.Equal(t, "river-meta-movie", name)
	assert.ElementsMatch(t, []string{ScopeSettingsTMDB, ScopeMediaWrite}, scopes)

	// A wrong key is rejected; a non-prefixed token isn't even looked up.
	_, _, ok = svc.AuthenticateServiceKey("rvk_wrong")
	assert.False(t, ok)
	_, _, ok = svc.AuthenticateServiceKey("not-a-key")
	assert.False(t, ok)
}

func TestServiceKey_MintRejectsDuplicateAndBadScope(t *testing.T) {
	svc := NewServiceKeyService(&memServiceKeyRepo{})
	_, _, err := svc.Mint("river-scan", []string{ScopeMediaWrite})
	require.NoError(t, err)

	_, _, err = svc.Mint("river-scan", []string{ScopeMediaWrite})
	assert.ErrorIs(t, err, apperrors.ErrConflict, "duplicate name is a conflict")

	_, _, err = svc.Mint("river-other", []string{"bogus:scope"})
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput, "unknown scope rejected")
}

func TestServiceKey_RevokeStopsAuth(t *testing.T) {
	repo := &memServiceKeyRepo{}
	svc := NewServiceKeyService(repo)
	plaintext, view, err := svc.Mint("river-scan", []string{ScopeMediaWrite})
	require.NoError(t, err)

	_, _, ok := svc.AuthenticateServiceKey(plaintext)
	require.True(t, ok)

	require.NoError(t, svc.Revoke(view.ID))
	_, _, ok = svc.AuthenticateServiceKey(plaintext)
	assert.False(t, ok, "revoked key no longer authenticates")
}

func TestServiceKey_SeedIsIdempotentAndDerivesScopes(t *testing.T) {
	repo := &memServiceKeyRepo{}
	svc := NewServiceKeyService(repo)

	// Scopes omitted → derived from the known service name.
	created, err := svc.Seed("river-meta-tv", "rvk_seededkey", nil)
	require.NoError(t, err)
	assert.True(t, created)

	k, err := repo.FindByName("river-meta-tv")
	require.NoError(t, err)
	assert.ElementsMatch(t, DefaultScopesForService("river-meta-tv"), parseScopes(k.Scopes))

	// Re-seeding the same name is a no-op (never clobbers).
	created, err = svc.Seed("river-meta-tv", "rvk_differentkey", nil)
	require.NoError(t, err)
	assert.False(t, created)

	// The originally seeded key still authenticates (not overwritten).
	name, _, ok := svc.AuthenticateServiceKey("rvk_seededkey")
	assert.True(t, ok)
	assert.Equal(t, "river-meta-tv", name)
}

func TestServiceKey_SeedRejectsMissingPrefix(t *testing.T) {
	svc := NewServiceKeyService(&memServiceKeyRepo{})
	_, err := svc.Seed("river-scan", "nope-no-prefix", nil)
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
}

func TestServiceKey_SeedEmptyIsSkipped(t *testing.T) {
	svc := NewServiceKeyService(&memServiceKeyRepo{})
	created, err := svc.Seed("river-scan", "", nil)
	require.NoError(t, err)
	assert.False(t, created)
}

func TestDefaultScopes_TranscoderCannotReadTMDB(t *testing.T) {
	// The headline security property: a transcoder's default scopes exclude the
	// TMDB secret; only the meta services carry it.
	assert.NotContains(t, DefaultScopesForService("river-video-trans"), ScopeSettingsTMDB)
	assert.NotContains(t, DefaultScopesForService("river-audio-trans"), ScopeSettingsTMDB)
	assert.Contains(t, DefaultScopesForService("river-meta-movie"), ScopeSettingsTMDB)
	assert.Contains(t, DefaultScopesForService("river-meta-tv"), ScopeSettingsTMDB)
}
