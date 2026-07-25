package services

import (
	"testing"

	"river-api/internal/apperrors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memSettingRepo struct{ m map[string]string }

func (r *memSettingRepo) Get(key string) (string, error) {
	if v, ok := r.m[key]; ok {
		return v, nil
	}
	return "", apperrors.ErrNotFound
}
func (r *memSettingRepo) Set(key, value string) error {
	if r.m == nil {
		r.m = map[string]string{}
	}
	r.m[key] = value
	return nil
}
func (r *memSettingRepo) SetIfAbsent(key, value string) (bool, error) {
	if r.m == nil {
		r.m = map[string]string{}
	}
	if _, ok := r.m[key]; ok {
		return false, nil
	}
	r.m[key] = value
	return true, nil
}

func TestSettingsService_RadarrConfig_TrimsAndEnables(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	svc := NewSettingsService(repo)

	_, _, enabled := svc.RadarrConfig()
	assert.False(t, enabled, "no URL => disabled")

	repo.Set(keyRadarrURL, "http://radarr:7878/")
	repo.Set(keyRadarrKey, "abc")
	url, key, enabled := svc.RadarrConfig()
	assert.Equal(t, "http://radarr:7878", url, "trailing slash trimmed")
	assert.Equal(t, "abc", key)
	assert.True(t, enabled)
}

func TestSettingsService_Integrations_MasksKeys(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	repo.Set(keyRadarrURL, "http://radarr")
	repo.Set(keyRadarrKey, "supersecret")
	svc := NewSettingsService(repo)

	s := svc.Integrations()
	assert.Equal(t, "http://radarr", s.RadarrURL)
	assert.True(t, s.RadarrHasKey, "key present => has_key true")
	assert.False(t, s.SonarrHasKey, "sonarr unset => has_key false")
	// The masked view must never carry the raw key.
	assert.NotContains(t, s.RadarrURL, "supersecret")
}

func TestSettingsService_Update_EmptyKeyPreservesExisting(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{keyRadarrKey: "oldkey"}}
	svc := NewSettingsService(repo)

	// URL set, empty key must not wipe the stored key.
	require.NoError(t, svc.UpdateIntegrations("http://new", "", "", ""))
	url, key, _ := svc.RadarrConfig()
	assert.Equal(t, "http://new", url)
	assert.Equal(t, "oldkey", key, "empty key on update preserves the existing key")

	// A non-empty key replaces it.
	require.NoError(t, svc.UpdateIntegrations("http://new", "newkey", "", ""))
	_, key2, _ := svc.RadarrConfig()
	assert.Equal(t, "newkey", key2)
}

func TestSettingsService_Seed_DoesNotOverwriteExisting(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{keyRadarrURL: "http://existing"}}
	svc := NewSettingsService(repo)

	require.NoError(t, svc.SeedIntegrations("http://from-env", "envkey", "http://sonarr", "sonarrkey"))

	ru, rk, _ := svc.RadarrConfig()
	assert.Equal(t, "http://existing", ru, "an already-set value is never overwritten by seed")
	assert.Equal(t, "envkey", rk, "an absent value is seeded")

	su, sk, _ := svc.SonarrConfig()
	assert.Equal(t, "http://sonarr", su)
	assert.Equal(t, "sonarrkey", sk)
}

func TestSettingsService_Seed_SkipsEmptyValues(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	svc := NewSettingsService(repo)

	require.NoError(t, svc.SeedIntegrations("", "", "", ""))
	_, _, radarrEnabled := svc.RadarrConfig()
	_, _, sonarrEnabled := svc.SonarrConfig()
	assert.False(t, radarrEnabled)
	assert.False(t, sonarrEnabled)
}
