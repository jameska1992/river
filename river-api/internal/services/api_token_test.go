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

type memAPITokenRepo struct{ toks []*models.APIToken }

func (m *memAPITokenRepo) Create(t *models.APIToken) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	m.toks = append(m.toks, t)
	return nil
}
func (m *memAPITokenRepo) FindByHash(hash string) (*models.APIToken, error) {
	for _, t := range m.toks {
		if t.TokenHash == hash && !t.Revoked {
			return t, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memAPITokenRepo) List() ([]models.APIToken, error) {
	out := make([]models.APIToken, len(m.toks))
	for i, t := range m.toks {
		out[i] = *t
	}
	return out, nil
}
func (m *memAPITokenRepo) Revoke(id string) error {
	for _, t := range m.toks {
		if t.ID.String() == id {
			t.Revoked = true
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (m *memAPITokenRepo) TouchLastUsed(id string, at time.Time) error { return nil }

func TestAPIToken_MintAndAuthenticate(t *testing.T) {
	svc := NewAPITokenService(&memAPITokenRepo{})
	owner := uuid.New()

	plaintext, view, err := svc.Mint(owner, "Homepage dashboard", nil, nil)
	require.NoError(t, err)
	assert.Contains(t, plaintext, APITokenPrefix)
	assert.Equal(t, []string{"read"}, view.Scopes, "v1 default scope is read")

	uid, scopes, ok := svc.AuthenticateAPIToken(plaintext)
	assert.True(t, ok)
	assert.Equal(t, owner.String(), uid)
	assert.Equal(t, []string{"read"}, scopes)

	_, _, ok = svc.AuthenticateAPIToken("rvat_wrong")
	assert.False(t, ok)
	_, _, ok = svc.AuthenticateAPIToken("not-a-token")
	assert.False(t, ok, "non-prefixed token isn't looked up")
}

func TestAPIToken_ExpiredRejected(t *testing.T) {
	svc := NewAPITokenService(&memAPITokenRepo{})
	past := time.Now().Add(-time.Hour)
	plaintext, _, err := svc.Mint(uuid.New(), "old", nil, &past)
	require.NoError(t, err)
	_, _, ok := svc.AuthenticateAPIToken(plaintext)
	assert.False(t, ok, "expired token must not authenticate")
}

func TestAPIToken_RevokeStopsAuth(t *testing.T) {
	repo := &memAPITokenRepo{}
	svc := NewAPITokenService(repo)
	plaintext, view, err := svc.Mint(uuid.New(), "temp", nil, nil)
	require.NoError(t, err)
	_, _, ok := svc.AuthenticateAPIToken(plaintext)
	require.True(t, ok)

	require.NoError(t, svc.Revoke(view.ID))
	_, _, ok = svc.AuthenticateAPIToken(plaintext)
	assert.False(t, ok)
}

func TestAPIToken_MintRequiresName(t *testing.T) {
	svc := NewAPITokenService(&memAPITokenRepo{})
	_, _, err := svc.Mint(uuid.New(), "  ", nil, nil)
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
}

func TestAPIToken_List(t *testing.T) {
	svc := NewAPITokenService(&memAPITokenRepo{})
	_, _, err := svc.Mint(uuid.New(), "one", nil, nil)
	require.NoError(t, err)
	_, _, err = svc.Mint(uuid.New(), "two", nil, nil)
	require.NoError(t, err)

	list, err := svc.List()
	require.NoError(t, err)
	require.Len(t, list, 2)
	// Views never carry the hash — only the display prefix.
	for _, v := range list {
		assert.NotEmpty(t, v.TokenPrefix)
	}
}
