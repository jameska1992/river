package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	RabbitMQURL string
	// RiverAPIURL + credentials: river-events reads media records back from
	// river-api to compute the transcode+enrich join.
	RiverAPIURL      string
	RiverAPIUsername string
	RiverAPIPassword string
	// RiverAPIKey is an optional per-service API key (service-role Phase 2).
	// When set it's used instead of password login; unset falls back to
	// username/password. Mirrors the other services.
	RiverAPIKey  string
	WorkerCount  int
	MaxRetries   int
	RetryBackoff time.Duration
}

func Load() (*Config, error) {
	username := os.Getenv("RIVER_API_USERNAME")
	password := os.Getenv("RIVER_API_PASSWORD")
	apiKey := os.Getenv("RIVER_API_KEY")
	// Either a key or a username/password pair must be present.
	if apiKey == "" && (username == "" || password == "") {
		return nil, fmt.Errorf("RIVER_API_KEY, or RIVER_API_USERNAME + RIVER_API_PASSWORD, is required")
	}
	return &Config{
		RabbitMQURL:      getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RiverAPIURL:      getEnv("RIVER_API_URL", "http://localhost:8080"),
		RiverAPIUsername: username,
		RiverAPIPassword: password,
		RiverAPIKey:      apiKey,
		WorkerCount:      getIntEnv("WORKER_COUNT", 2),
		MaxRetries:       getIntEnv("MAX_RETRIES", 5),
		RetryBackoff:     time.Duration(getIntEnv("RETRY_BACKOFF_SECONDS", 30)) * time.Second,
	}, nil
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
