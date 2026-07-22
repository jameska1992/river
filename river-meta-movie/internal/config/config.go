package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	RabbitMQURL      string
	RabbitMQExchange string
	RiverAPIURL      string
	RiverAPIUsername string
	RiverAPIPassword string
	TMDBAPIKey       string
	TMDBImageBase    string
	WorkerCount      int
	Port             string
}

func Load() (*Config, error) {
	cfg := &Config{
		RabbitMQURL:      getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQExchange: getEnv("RABBITMQ_EXCHANGE", "river.media"),
		RiverAPIURL:      getEnv("RIVER_API_URL", "http://localhost:8080"),
		RiverAPIUsername: os.Getenv("RIVER_API_USERNAME"),
		RiverAPIPassword: os.Getenv("RIVER_API_PASSWORD"),
		TMDBAPIKey:       os.Getenv("TMDB_API_KEY"),
		TMDBImageBase:    getEnv("TMDB_IMAGE_BASE", "https://image.tmdb.org/t/p/original"),
		WorkerCount:      getEnvInt("WORKER_COUNT", 2),
		Port:             getEnv("PORT", "8081"),
	}
	if cfg.RiverAPIUsername == "" || cfg.RiverAPIPassword == "" {
		return nil, fmt.Errorf("RIVER_API_USERNAME and RIVER_API_PASSWORD are required")
	}
	if cfg.TMDBAPIKey == "" {
		return nil, fmt.Errorf("TMDB_API_KEY is required")
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			return i
		}
	}
	return def
}
