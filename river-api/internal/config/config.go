package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	DatabaseURL        string
	JWTSecret          string
	JWTAccessExpiry    time.Duration
	JWTRefreshExpiry   time.Duration
	// JWTStreamExpiry sets the lifetime of the media-stream JWT embedded
	// in <video> src URLs. Has to comfortably exceed the longest movie /
	// episode the user might watch in one sitting, because the browser
	// can't refresh a token mid-playback. Default 8h — covers any single
	// feature-length viewing session.
	JWTStreamExpiry    time.Duration
	MediaBasePath     string
	RiverScanURL      string
	RiverMetaMovieURL string
	RiverMetaTVURL    string
	RiverMetaBookURL  string
	RiverMetaMusicURL string
	RadarrURL         string
	RadarrAPIKey      string
	SonarrURL         string
	SonarrAPIKey      string
	// Comma-separated origin list parsed from CORS_ALLOWED_ORIGINS.
	// The single literal "*" (the default) is treated specially as
	// allow-all — convenient for dev where the river-tv WebView and
	// river-web nginx sit on different hosts/ports. Production
	// deployments should set this to the actual frontend origins.
	CORSAllowedOrigins []string
}

func Load() *Config {
	return &Config{
		Port:              getEnv("PORT", "8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://river:river@localhost:5432/river?sslmode=disable"),
		JWTSecret:         getEnv("JWT_SECRET", "change-me-in-production"),
		JWTAccessExpiry:   time.Duration(getIntEnv("JWT_ACCESS_EXPIRY_MINUTES", 15)) * time.Minute,
		JWTRefreshExpiry:  time.Duration(getIntEnv("JWT_REFRESH_EXPIRY_DAYS", 7)) * 24 * time.Hour,
		JWTStreamExpiry:   time.Duration(getIntEnv("JWT_STREAM_EXPIRY_HOURS", 8)) * time.Hour,
		MediaBasePath:     getEnv("MEDIA_BASE_PATH", "/media"),
		RiverScanURL:      getEnv("RIVER_SCAN_URL", ""),
		RiverMetaMovieURL: getEnv("RIVER_META_MOVIE_URL", ""),
		RiverMetaTVURL:    getEnv("RIVER_META_TV_URL", ""),
		RiverMetaBookURL:  getEnv("RIVER_META_BOOK_URL", ""),
		RiverMetaMusicURL: getEnv("RIVER_META_MUSIC_URL", ""),
		RadarrURL:         getEnv("RADARR_URL", ""),
		RadarrAPIKey:      getEnv("RADARR_API_KEY", ""),
		SonarrURL:         getEnv("SONARR_URL", ""),
		SonarrAPIKey:      getEnv("SONARR_API_KEY", ""),
		CORSAllowedOrigins: parseCSV(getEnv("CORS_ALLOWED_ORIGINS", "*")),
	}
}

// parseCSV splits a comma-separated env value, trimming whitespace and
// dropping empty entries.
func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
