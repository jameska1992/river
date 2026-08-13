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

func TestSettingsHandler_Metadata(t *testing.T) {
	repo := &fakeSettingRepo{m: map[string]string{}}
	gin.SetMode(gin.TestMode)
	h := NewSettingsHandler(services.NewSettingsService(repo))
	r := gin.New()
	r.GET("/admin/settings/metadata", h.GetMetadata)
	r.PUT("/admin/settings/metadata", h.UpdateMetadata)
	r.GET("/admin/settings/tmdb", h.GetTMDBKey)

	// Initially unset.
	w := doJSON(r, http.MethodGet, "/admin/settings/metadata", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"tmdb_has_key":false`)

	// Set the key.
	w = doJSON(r, http.MethodPut, "/admin/settings/metadata", `{"tmdb_api_key":"k-abc"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"tmdb_has_key":true`)
	assert.NotContains(t, w.Body.String(), "k-abc", "masked view must not leak the key")
	assert.Equal(t, "k-abc", repo.m["tmdb.api_key"])

	// The raw service endpoint returns the actual key.
	w = doJSON(r, http.MethodGet, "/admin/settings/tmdb", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "k-abc")

	// Empty update preserves.
	w = doJSON(r, http.MethodPut, "/admin/settings/metadata", `{"tmdb_api_key":""}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "k-abc", repo.m["tmdb.api_key"])
}

func TestSettingsHandler_Transcoding(t *testing.T) {
	repo := &fakeSettingRepo{m: map[string]string{}}
	gin.SetMode(gin.TestMode)
	h := NewSettingsHandler(services.NewSettingsService(repo))
	r := gin.New()
	r.GET("/admin/settings/transcoding", h.GetTranscoding)
	r.PUT("/admin/settings/transcoding", h.UpdateTranscoding)

	// Unset install returns today's defaults.
	w := doJSON(r, http.MethodGet, "/admin/settings/transcoding", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"nvenc_preset":"p3"`)
	assert.Contains(t, w.Body.String(), `"music_bitrate":256`)

	// Invalid value -> 400 with a descriptive message, nothing persisted.
	w = doJSON(r, http.MethodPut, "/admin/settings/transcoding",
		`{"max_height":1080,"quality":99,"nvenc_preset":"p3","x264_preset":"medium","force_cpu":false,"audio_bitrate":192,"music_bitrate":256}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "quality")
	assert.Empty(t, repo.m, "no keys written on a rejected update")

	// Valid update -> 200 + persisted, echoed back resolved.
	w = doJSON(r, http.MethodPut, "/admin/settings/transcoding",
		`{"max_height":2160,"quality":20,"nvenc_preset":"p5","x264_preset":"slow","force_cpu":true,"audio_bitrate":256,"music_bitrate":320}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2160", repo.m["transcoding.max_height"])
	assert.Equal(t, "true", repo.m["transcoding.force_cpu"])
	assert.Equal(t, "320", repo.m["transcoding.music_bitrate"])
	assert.Contains(t, w.Body.String(), `"x264_preset":"slow"`)
}

func TestSettingsHandler_Scanning(t *testing.T) {
	repo := &fakeSettingRepo{m: map[string]string{}}
	gin.SetMode(gin.TestMode)
	h := NewSettingsHandler(services.NewSettingsService(repo))
	r := gin.New()
	r.GET("/admin/settings/scanning", h.GetScanning)
	r.PUT("/admin/settings/scanning", h.UpdateScanning)

	// invalid duration -> 400
	w := doJSON(r, http.MethodPut, "/admin/settings/scanning", `{"scan_interval":"soon"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// missing -> 400 (binding required)
	w = doJSON(r, http.MethodPut, "/admin/settings/scanning", `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// valid -> 200 + persisted
	w = doJSON(r, http.MethodPut, "/admin/settings/scanning", `{"scan_interval":"45m"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "45m")
	assert.Equal(t, "45m", repo.m["scan.interval"])

	w = doJSON(r, http.MethodGet, "/admin/settings/scanning", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "45m")
}
