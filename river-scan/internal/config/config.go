package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	APIBaseURL          string
	APIUsername         string
	APIPassword         string
	RabbitMQURL         string
	RabbitMQExchange    string
	StatePath           string
	HTTPPort            string
	DisableTranscoding  bool
	MaxScanDepth        int
}

func Load() (*Config, error) {
	username := os.Getenv("RIVER_API_USERNAME")
	if username == "" {
		return nil, fmt.Errorf("RIVER_API_USERNAME is required")
	}
	password := os.Getenv("RIVER_API_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("RIVER_API_PASSWORD is required")
	}

	// SCAN_INTERVAL is no longer read here — it lives in river-api's
	// settings store (seeded from the env var on first boot) and is
	// fetched at runtime so it can change without a restart.

	maxDepth := 0 // 0 means "use scanner default"
	if s := os.Getenv("MAX_SCAN_DEPTH"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid MAX_SCAN_DEPTH %q", s)
		}
		maxDepth = n
	}

	return &Config{
		APIBaseURL:         getEnv("RIVER_API_URL", "http://localhost:8080"),
		APIUsername:        username,
		APIPassword:        password,
		RabbitMQURL:        getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQExchange:   getEnv("RABBITMQ_EXCHANGE", "river.media"),
		StatePath:          getEnv("STATE_PATH", "scanner-state.json"),
		HTTPPort:           getEnv("HTTP_PORT", "8081"),
		DisableTranscoding: os.Getenv("DISABLE_TRANSCODING") != "",
		MaxScanDepth:       maxDepth,
	}, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
