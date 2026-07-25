package services

import (
	"strings"

	"river-api/internal/repository"
)

// Setting keys. Namespaced so future config areas coexist in one table.
const (
	keyRadarrURL = "radarr.url"
	keyRadarrKey = "radarr.api_key"
	keySonarrURL = "sonarr.url"
	keySonarrKey = "sonarr.api_key"
	keyTMDBKey      = "tmdb.api_key"
	keyScanInterval = "scan.interval"
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
