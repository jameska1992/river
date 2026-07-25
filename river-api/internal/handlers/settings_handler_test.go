package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"river-api/internal/apperrors"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSettingRepo struct{ m map[string]string }

func (r *fakeSettingRepo) Get(key string) (string, error) {
	if v, ok := r.m[key]; ok {
		return v, nil
	}
	return "", apperrors.ErrNotFound
}
func (r *fakeSettingRepo) Set(key, value string) error {
	if r.m == nil {
		r.m = map[string]string{}
	}
	r.m[key] = value
	return nil
}
func (r *fakeSettingRepo) SetIfAbsent(key, value string) (bool, error) {
	if r.m == nil {
		r.m = map[string]string{}
	}
	if _, ok := r.m[key]; ok {
		return false, nil
	}
	r.m[key] = value
	return true, nil
}

func settingsRouter(repo *fakeSettingRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSettingsHandler(services.NewSettingsService(repo))
	r := gin.New()
	r.GET("/admin/settings/integrations", h.GetIntegrations)
	r.PUT("/admin/settings/integrations", h.UpdateIntegrations)
	r.POST("/admin/settings/integrations/seed", h.SeedIntegrations)
	return r
}

func TestSettingsHandler_Get_MasksSecrets(t *testing.T) {
	repo := &fakeSettingRepo{m: map[string]string{"radarr.url": "http://radarr", "radarr.api_key": "supersecret"}}
	r := settingsRouter(repo)

	w := doJSON(r, http.MethodGet, "/admin/settings/integrations", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "supersecret", "the raw API key must never be serialized")

	var body struct {
		RadarrURL    string `json:"radarr_url"`
		RadarrHasKey bool   `json:"radarr_has_key"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "http://radarr", body.RadarrURL)
	assert.True(t, body.RadarrHasKey)
}

func TestSettingsHandler_Update(t *testing.T) {
	repo := &fakeSettingRepo{m: map[string]string{}}
	r := settingsRouter(repo)

	w := doJSON(r, http.MethodPut, "/admin/settings/integrations",
		`{"radarr_url":"http://radarr:7878","radarr_api_key":"k1","sonarr_url":"","sonarr_api_key":""}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://radarr:7878", repo.m["radarr.url"])
	assert.Equal(t, "k1", repo.m["radarr.api_key"])

	// Empty key on a subsequent update preserves the stored key.
	w = doJSON(r, http.MethodPut, "/admin/settings/integrations",
		`{"radarr_url":"http://radarr:7878","radarr_api_key":"","sonarr_url":"","sonarr_api_key":""}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "k1", repo.m["radarr.api_key"], "empty key preserved existing")
}

func TestSettingsHandler_Seed_SetsIfAbsent(t *testing.T) {
	repo := &fakeSettingRepo{m: map[string]string{"radarr.url": "http://existing"}}
	r := settingsRouter(repo)

	w := doJSON(r, http.MethodPost, "/admin/settings/integrations/seed",
		`{"radarr_url":"http://from-env","radarr_api_key":"envkey","sonarr_url":"http://sonarr","sonarr_api_key":"sk"}`)
	require.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, "http://existing", repo.m["radarr.url"], "seed must not overwrite an existing value")
	assert.Equal(t, "envkey", repo.m["radarr.api_key"], "seed fills an absent value")
	assert.Equal(t, "http://sonarr", repo.m["sonarr.url"])
}
