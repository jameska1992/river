package services

import (
	"fmt"
	"strconv"
	"strings"

	"river-api/internal/apperrors"
	"river-api/internal/repository"
)

// Setting keys. Namespaced so future config areas coexist in one table.
const (
	keyRadarrURL    = "radarr.url"
	keyRadarrKey    = "radarr.api_key"
	keySonarrURL    = "sonarr.url"
	keySonarrKey    = "sonarr.api_key"
	keyTMDBKey      = "tmdb.api_key"
	keyScanInterval = "scan.interval"

	keyTransMaxHeight    = "transcoding.max_height"
	keyTransQuality      = "transcoding.quality"
	keyTransNVENCPreset  = "transcoding.nvenc_preset"
	keyTransX264Preset   = "transcoding.x264_preset"
	keyTransForceCPU     = "transcoding.force_cpu"
	keyTransAudioBitrate = "transcoding.audio_bitrate"
	keyTransMusicBitrate = "transcoding.music_bitrate"
)

// Transcoding defaults. These must match the compiled-in constants the
// transcoders use today, so an unconfigured install behaves identically
// (river-video-trans: NVENC p3/cq23, libx264 medium/crf23, 1080p cap,
// AAC 192k; river-audio-trans: AAC 256k).
const (
	defaultTransMaxHeight    = 1080
	defaultTransQuality      = 23
	defaultTransNVENCPreset  = "p3"
	defaultTransX264Preset   = "medium"
	defaultTransForceCPU     = false
	defaultTransAudioBitrate = 192
	defaultTransMusicBitrate = 256
)

// Allowed values for the structured transcoding knobs. Validation is
// strictly enum/range based — no admin input is ever passed through as a
// raw ffmpeg argument, so a bad value can't inject flags or break jobs.
var (
	transMaxHeights   = []int{0, 720, 1080, 2160} // 0 = no cap
	transNVENCPresets = []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7"}
	transX264Presets  = []string{
		"ultrafast", "superfast", "veryfast", "faster", "fast",
		"medium", "slow", "slower", "veryslow",
	}
	transBitrates = []int{128, 192, 256, 320} // kbps
)

type SettingsService struct {
	repo repository.SettingRepository
}

func NewSettingsService(repo repository.SettingRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

// get returns the stored value or "" if unset (or on error — callers
// treat an unreadable setting the same as unconfigured).
func (s *SettingsService) get(key string) string {
	v, err := s.repo.Get(key)
	if err != nil {
		return ""
	}
	return v
}

// RadarrConfig returns the current Radarr base URL (trailing slash
// trimmed), API key, and whether Radarr is configured (URL non-empty).
func (s *SettingsService) RadarrConfig() (url, key string, enabled bool) {
	url = strings.TrimRight(strings.TrimSpace(s.get(keyRadarrURL)), "/")
	key = s.get(keyRadarrKey)
	return url, key, url != ""
}

// SonarrConfig mirrors RadarrConfig for Sonarr.
func (s *SettingsService) SonarrConfig() (url, key string, enabled bool) {
	url = strings.TrimRight(strings.TrimSpace(s.get(keySonarrURL)), "/")
	key = s.get(keySonarrKey)
	return url, key, url != ""
}

// TMDBKey returns the raw TMDB API key (empty when unset). Unlike the
// integration views this is not masked — it's consumed by the metadata
// services (river-meta-movie / river-meta-tv), which authenticate as
// admin and need the actual key to call TMDB.
func (s *SettingsService) TMDBKey() string {
	return s.get(keyTMDBKey)
}

// MetadataSettings is the admin-facing (masked) view of metadata config.
type MetadataSettings struct {
	TMDBHasKey bool `json:"tmdb_has_key"`
}

func (s *SettingsService) Metadata() MetadataSettings {
	return MetadataSettings{TMDBHasKey: s.get(keyTMDBKey) != ""}
}

// UpdateMetadata sets the TMDB key when a non-empty value is supplied;
// an empty value leaves the stored key untouched (same convention as the
// integration API keys).
func (s *SettingsService) UpdateMetadata(tmdbKey string) error {
	if tmdbKey == "" {
		return nil
	}
	return s.repo.Set(keyTMDBKey, tmdbKey)
}

// ScanningSettings is the admin-facing view of scan config. The interval
// is a Go duration string (e.g. "1h", "30m", "3600s"); not a secret, so
// it's returned as-is.
type ScanningSettings struct {
	ScanInterval string `json:"scan_interval"`
}

func (s *SettingsService) Scanning() ScanningSettings {
	return ScanningSettings{ScanInterval: s.get(keyScanInterval)}
}

// ScanInterval returns the raw scan-interval duration string (empty when
// unset).
func (s *SettingsService) ScanInterval() string {
	return s.get(keyScanInterval)
}

// UpdateScanning stores the scan interval. Callers (handler) are expected
// to have validated it as a parseable duration.
func (s *SettingsService) UpdateScanning(scanInterval string) error {
	return s.repo.Set(keyScanInterval, strings.TrimSpace(scanInterval))
}

// TranscodingSettings is the resolved transcoding configuration: every
// field carries either the stored value or the compiled-in default, so
// the same struct serves both the admin editor and the transcoders that
// read it at job time. None of these are secrets, so nothing is masked.
type TranscodingSettings struct {
	MaxHeight    int    `json:"max_height"`    // 0 = no cap; else 720/1080/2160
	Quality      int    `json:"quality"`       // 0..51, mapped to NVENC -cq and x264 -crf
	NVENCPreset  string `json:"nvenc_preset"`  // p1..p7
	X264Preset   string `json:"x264_preset"`   // ultrafast..veryslow
	ForceCPU     bool   `json:"force_cpu"`     // skip the NVENC path even with a GPU
	AudioBitrate int    `json:"audio_bitrate"` // kbps, river-video-trans audio path
	MusicBitrate int    `json:"music_bitrate"` // kbps, river-audio-trans
}

// getInt returns the stored value for key parsed as an int, or def when
// the key is unset or unparseable — an unreadable setting is treated as
// unconfigured, same as get().
func (s *SettingsService) getInt(key string, def int) int {
	v := s.get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// getString returns the stored value for key, or def when unset.
func (s *SettingsService) getString(key, def string) string {
	if v := s.get(key); v != "" {
		return v
	}
	return def
}

// getBool returns the stored value for key parsed as a bool, or def when
// unset or unparseable.
func (s *SettingsService) getBool(key string, def bool) bool {
	v := s.get(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// Transcoding returns the effective transcoding config, filling any unset
// (or corrupt) key with its default so callers never see a zero value
// that wasn't deliberately configured.
func (s *SettingsService) Transcoding() TranscodingSettings {
	return TranscodingSettings{
		MaxHeight:    s.getInt(keyTransMaxHeight, defaultTransMaxHeight),
		Quality:      s.getInt(keyTransQuality, defaultTransQuality),
		NVENCPreset:  s.getString(keyTransNVENCPreset, defaultTransNVENCPreset),
		X264Preset:   s.getString(keyTransX264Preset, defaultTransX264Preset),
		ForceCPU:     s.getBool(keyTransForceCPU, defaultTransForceCPU),
		AudioBitrate: s.getInt(keyTransAudioBitrate, defaultTransAudioBitrate),
		MusicBitrate: s.getInt(keyTransMusicBitrate, defaultTransMusicBitrate),
	}
}

// UpdateTranscoding validates every field against its allowed enum/range
// and, only if all pass, writes them. Validation failures wrap
// ErrInvalidInput (→ 400) and nothing is persisted, so a rejected request
// leaves the stored config untouched.
func (s *SettingsService) UpdateTranscoding(in TranscodingSettings) error {
	if !containsInt(transMaxHeights, in.MaxHeight) {
		return fmt.Errorf("%w: max_height must be one of 0, 720, 1080, 2160", apperrors.ErrInvalidInput)
	}
	if in.Quality < 0 || in.Quality > 51 {
		return fmt.Errorf("%w: quality must be between 0 and 51", apperrors.ErrInvalidInput)
	}
	if !containsString(transNVENCPresets, in.NVENCPreset) {
		return fmt.Errorf("%w: nvenc_preset must be one of p1..p7", apperrors.ErrInvalidInput)
	}
	if !containsString(transX264Presets, in.X264Preset) {
		return fmt.Errorf("%w: x264_preset must be a valid libx264 preset (ultrafast..veryslow)", apperrors.ErrInvalidInput)
	}
	if !containsInt(transBitrates, in.AudioBitrate) {
		return fmt.Errorf("%w: audio_bitrate must be one of 128, 192, 256, 320", apperrors.ErrInvalidInput)
	}
	if !containsInt(transBitrates, in.MusicBitrate) {
		return fmt.Errorf("%w: music_bitrate must be one of 128, 192, 256, 320", apperrors.ErrInvalidInput)
	}

	writes := []struct {
		key, value string
	}{
		{keyTransMaxHeight, strconv.Itoa(in.MaxHeight)},
		{keyTransQuality, strconv.Itoa(in.Quality)},
		{keyTransNVENCPreset, in.NVENCPreset},
		{keyTransX264Preset, in.X264Preset},
		{keyTransForceCPU, strconv.FormatBool(in.ForceCPU)},
		{keyTransAudioBitrate, strconv.Itoa(in.AudioBitrate)},
		{keyTransMusicBitrate, strconv.Itoa(in.MusicBitrate)},
	}
	for _, w := range writes {
		if err := s.repo.Set(w.key, w.value); err != nil {
			return err
		}
	}
	return nil
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsString(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// IntegrationSettings is the admin-facing view. Secrets are never
// returned — only whether a key is set.
type IntegrationSettings struct {
	RadarrURL    string `json:"radarr_url"`
	RadarrHasKey bool   `json:"radarr_has_key"`
	SonarrURL    string `json:"sonarr_url"`
	SonarrHasKey bool   `json:"sonarr_has_key"`
}

func (s *SettingsService) Integrations() IntegrationSettings {
	ru, rk, _ := s.RadarrConfig()
	su, sk, _ := s.SonarrConfig()
	return IntegrationSettings{
		RadarrURL:    ru,
		RadarrHasKey: rk != "",
		SonarrURL:    su,
		SonarrHasKey: sk != "",
	}
}

// UpdateIntegrations applies an admin edit. URLs are always written
// (an empty URL clears/disables that integration). API keys are only
// written when non-empty, so an admin can change a URL without having to
// re-enter the key; the key is left untouched otherwise.
func (s *SettingsService) UpdateIntegrations(radarrURL, radarrKey, sonarrURL, sonarrKey string) error {
	if err := s.repo.Set(keyRadarrURL, strings.TrimSpace(radarrURL)); err != nil {
		return err
	}
	if radarrKey != "" {
		if err := s.repo.Set(keyRadarrKey, radarrKey); err != nil {
			return err
		}
	}
	if err := s.repo.Set(keySonarrURL, strings.TrimSpace(sonarrURL)); err != nil {
		return err
	}
	if sonarrKey != "" {
		if err := s.repo.Set(keySonarrKey, sonarrKey); err != nil {
			return err
		}
	}
	return nil
}

// SeedIntegrations writes each provided non-empty value only if that key
// is not already set in the DB. Used by the deployment init step to
// bootstrap from environment variables without ever overwriting values
// an admin has changed via the UI.
func (s *SettingsService) SeedIntegrations(radarrURL, radarrKey, sonarrURL, sonarrKey, tmdbKey, scanInterval string) error {
	seeds := []struct{ key, value string }{
		{keyRadarrURL, strings.TrimSpace(radarrURL)},
		{keyRadarrKey, radarrKey},
		{keySonarrURL, strings.TrimSpace(sonarrURL)},
		{keySonarrKey, sonarrKey},
		{keyTMDBKey, tmdbKey},
		{keyScanInterval, strings.TrimSpace(scanInterval)},
	}
	for _, sd := range seeds {
		if sd.value == "" {
			continue
		}
		if _, err := s.repo.SetIfAbsent(sd.key, sd.value); err != nil {
			return err
		}
	}
	return nil
}
