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

	require.NoError(t, svc.SeedIntegrations("http://from-env", "envkey", "http://sonarr", "sonarrkey", "tmdbkey", "1h"))

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

	require.NoError(t, svc.SeedIntegrations("", "", "", "", "", ""))
	_, _, radarrEnabled := svc.RadarrConfig()
	_, _, sonarrEnabled := svc.SonarrConfig()
	assert.False(t, radarrEnabled)
	assert.False(t, sonarrEnabled)
}

func TestSettingsService_Metadata_MasksTMDBKey(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{keyTMDBKey: "secret-tmdb"}}
	svc := NewSettingsService(repo)

	assert.Equal(t, "secret-tmdb", svc.TMDBKey(), "raw key available for service use")
	m := svc.Metadata()
	assert.True(t, m.TMDBHasKey, "masked view reports the key is set")
}

func TestSettingsService_UpdateMetadata_EmptyPreserves(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{keyTMDBKey: "old"}}
	svc := NewSettingsService(repo)

	require.NoError(t, svc.UpdateMetadata(""))
	assert.Equal(t, "old", svc.TMDBKey(), "empty update preserves the stored key")

	require.NoError(t, svc.UpdateMetadata("new"))
	assert.Equal(t, "new", svc.TMDBKey())
}

func TestSettingsService_Seed_IncludesTMDB(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	svc := NewSettingsService(repo)
	require.NoError(t, svc.SeedIntegrations("", "", "", "", "seeded-tmdb", ""))
	assert.Equal(t, "seeded-tmdb", svc.TMDBKey())
}

func TestSettingsService_Scanning(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	svc := NewSettingsService(repo)

	assert.Equal(t, "", svc.Scanning().ScanInterval)
	require.NoError(t, svc.UpdateScanning("30m"))
	assert.Equal(t, "30m", svc.Scanning().ScanInterval)
	assert.Equal(t, "30m", svc.ScanInterval())
}

func TestSettingsService_Seed_IncludesScanInterval(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	svc := NewSettingsService(repo)
	require.NoError(t, svc.SeedIntegrations("", "", "", "", "", "1h"))
	assert.Equal(t, "1h", svc.ScanInterval())
}

func TestSettingsService_Transcoding_DefaultsMatchTodaysConstants(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	svc := NewSettingsService(repo)

	// An unconfigured install must behave exactly like the compiled-in
	// defaults so out-of-the-box transcoding is unchanged.
	got := svc.Transcoding()
	assert.Equal(t, TranscodingSettings{
		MaxHeight:    1080,
		Quality:      23,
		NVENCPreset:  "p3",
		X264Preset:   "medium",
		ForceCPU:     false,
		AudioBitrate: 192,
		MusicBitrate: 256,
	}, got)
}

func TestSettingsService_Transcoding_ReadsStoredValues(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	svc := NewSettingsService(repo)

	in := TranscodingSettings{
		MaxHeight:    2160,
		Quality:      18,
		NVENCPreset:  "p5",
		X264Preset:   "slow",
		ForceCPU:     true,
		AudioBitrate: 320,
		MusicBitrate: 128,
	}
	require.NoError(t, svc.UpdateTranscoding(in))
	assert.Equal(t, in, svc.Transcoding())
}

func TestSettingsService_Transcoding_NoCapIsPersisted(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{}}
	svc := NewSettingsService(repo)

	// 0 (no cap) is a valid, deliberate value — it must round-trip and not
	// be re-defaulted to 1080 as an unset key would be.
	in := svc.Transcoding()
	in.MaxHeight = 0
	require.NoError(t, svc.UpdateTranscoding(in))
	assert.Equal(t, 0, svc.Transcoding().MaxHeight)
}

func TestSettingsService_Transcoding_CorruptValueFallsBackToDefault(t *testing.T) {
	repo := &memSettingRepo{m: map[string]string{keyTransQuality: "not-a-number"}}
	svc := NewSettingsService(repo)
	assert.Equal(t, 23, svc.Transcoding().Quality, "unparseable stored value => default")
}

func TestSettingsService_UpdateTranscoding_RejectsInvalid(t *testing.T) {
	valid := TranscodingSettings{
		MaxHeight:    1080,
		Quality:      23,
		NVENCPreset:  "p3",
		X264Preset:   "medium",
		ForceCPU:     false,
		AudioBitrate: 192,
		MusicBitrate: 256,
	}
	cases := map[string]func(TranscodingSettings) TranscodingSettings{
		"max_height off-enum":    func(s TranscodingSettings) TranscodingSettings { s.MaxHeight = 900; return s },
		"quality below range":    func(s TranscodingSettings) TranscodingSettings { s.Quality = -1; return s },
		"quality above range":    func(s TranscodingSettings) TranscodingSettings { s.Quality = 52; return s },
		"nvenc preset off-enum":  func(s TranscodingSettings) TranscodingSettings { s.NVENCPreset = "p9"; return s },
		"x264 preset off-enum":   func(s TranscodingSettings) TranscodingSettings { s.X264Preset = "turbo"; return s },
		"audio bitrate off-enum": func(s TranscodingSettings) TranscodingSettings { s.AudioBitrate = 200; return s },
		"music bitrate off-enum": func(s TranscodingSettings) TranscodingSettings { s.MusicBitrate = 999; return s },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &memSettingRepo{m: map[string]string{}}
			svc := NewSettingsService(repo)
			err := svc.UpdateTranscoding(mutate(valid))
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidInput)
			// A rejected request must not persist any partial state.
			assert.Empty(t, repo.m, "no keys written on validation failure")
		})
	}
}
