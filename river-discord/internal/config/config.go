package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds river-discord's settings.
type Config struct {
	// Port is the HTTP listen port for the webhook receiver.
	Port string
	// WebhookPath is the route river-events POSTs lifecycle events to.
	WebhookPath string
	// WebhookSecret is the shared HMAC secret (the whsec_… value from the
	// river-api webhook) used to verify X-River-Signature. Required.
	WebhookSecret string

	// RiverAPIURL is the base URL of river-api, read for embed enrichment.
	RiverAPIURL string
	// RiverAPIToken is a read-only API token (rvat_… from #195) used as a
	// Bearer credential to read media records. Required.
	RiverAPIToken string

	// DiscordWebhookURL is the default Discord incoming-webhook URL — the
	// channel used when no per-library/type route matches. Required.
	DiscordWebhookURL string
	// DiscordRoutes maps a library id OR a media type (movie|tvshow|music|
	// audiobook) to a Discord webhook URL, so different libraries post to
	// different channels. Checked library-id first, then type, then default.
	DiscordRoutes map[string]string

	// BatchWindow coalesces a burst of ready events for the same title (e.g. a
	// season's episodes) into one message once things go quiet for this long.
	BatchWindow time.Duration
	// NotifyKinds is the set of lifecycle kinds that trigger a notification.
	// Defaults to media.ready only.
	NotifyKinds map[string]bool
}

func Load() (*Config, error) {
	secret := os.Getenv("RIVER_WEBHOOK_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("RIVER_WEBHOOK_SECRET is required (the whsec_… secret from the river-api webhook)")
	}
	token := os.Getenv("RIVER_API_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("RIVER_API_TOKEN is required (a read-only rvat_… API token for reading media records)")
	}
	discordURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordURL == "" {
		return nil, fmt.Errorf("DISCORD_WEBHOOK_URL is required (the default Discord channel webhook)")
	}
	routes, err := parseRoutes(os.Getenv("DISCORD_ROUTES"))
	if err != nil {
		return nil, fmt.Errorf("DISCORD_ROUTES: %w", err)
	}
	return &Config{
		Port:              getEnv("PORT", "8090"),
		WebhookPath:       getEnv("WEBHOOK_PATH", "/hooks/river"),
		WebhookSecret:     secret,
		RiverAPIURL:       getEnv("RIVER_API_URL", "http://localhost:8080"),
		RiverAPIToken:     token,
		DiscordWebhookURL: discordURL,
		DiscordRoutes:     routes,
		BatchWindow:       time.Duration(getIntEnv("BATCH_WINDOW_SECONDS", 10)) * time.Second,
		NotifyKinds:       parseKinds(getEnv("NOTIFY_KINDS", "media.ready")),
	}, nil
}

// parseRoutes decodes the optional DISCORD_ROUTES JSON object. Empty is fine.
func parseRoutes(s string) (map[string]string, error) {
	if strings.TrimSpace(s) == "" {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("expected a JSON object of {\"key\":\"url\"}: %w", err)
	}
	return out, nil
}

func parseKinds(s string) map[string]bool {
	out := map[string]bool{}
	for _, k := range strings.Split(s, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out[k] = true
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
