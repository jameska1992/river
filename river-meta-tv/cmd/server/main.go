package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"river-meta-tv/internal/apiclient"
	"river-meta-tv/internal/config"
	"river-meta-tv/internal/consumer"
	"river-meta-tv/internal/processor"
	"river-meta-tv/internal/tmdb"
)

// cachedTMDBKey returns a provider that fetches the TMDB API key from
// river-api, caching it for ttl. This keeps the key out of the service's
// environment and lets an admin change it (via the settings UI) without
// restarting — the new value is picked up on the next refresh — while
// avoiding a river-api lookup on every TMDB call.
func cachedTMDBKey(fetch func() (string, error), ttl time.Duration) func() string {
	var (
		mu        sync.Mutex
		key       string
		fetchedAt time.Time
	)
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		if key == "" || time.Since(fetchedAt) > ttl {
			if v, err := fetch(); err == nil && v != "" {
				key, fetchedAt = v, time.Now()
			} else if err != nil {
				log.Printf("WARN fetch TMDB key from river-api: %v", err)
			}
		}
		return key
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL config: %v", err)
	}

	api := apiclient.New(cfg.RiverAPIURL, cfg.RiverAPIUsername, cfg.RiverAPIPassword, "river-meta-tv")
	if err := api.Login(); err != nil {
		log.Fatalf("FATAL river-api login: %v", err)
	}
	log.Printf("INFO authenticated with river-api at %s", cfg.RiverAPIURL)
	api.Log("info", "started")

	tmdbClient := tmdb.New(cachedTMDBKey(api.GetTMDBKey, 5*time.Minute), cfg.TMDBImageBase)

	// Use one connection for initial exchange/queue setup, then close it.
	setupCons, err := consumer.New(cfg.RabbitMQURL, cfg.RabbitMQExchange)
	if err != nil {
		log.Fatalf("FATAL rabbitmq setup: %v", err)
	}
	setupCons.Close()
	log.Printf("INFO exchange and queue declared, exchange=%s", cfg.RabbitMQExchange)

	proc := processor.New(api, tmdbClient)

	// HTTP trigger server for on-demand metadata refresh.
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /refresh/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			// Body is optional. An empty body falls back to title-search;
			// {imdb_id: "tt..."} skips search via TMDB's /find lookup.
			var body struct {
				IMDBID string `json:"imdb_id"`
			}
			if r.Body != nil {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
					http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
					return
				}
			}
			if err := proc.RefreshByIDWithIMDB(id, body.IMDBID); err != nil {
				log.Printf("ERROR refresh show %s: %v", id, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		})
		log.Printf("INFO HTTP trigger server on :%s", cfg.Port)
		if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
			log.Printf("ERROR HTTP server: %v", err)
		}
	}()

	// Spin up WorkerCount independent consumers. RabbitMQ round-robins messages
	// across them; each holds its own channel with QoS prefetch=1.
	errCh := make(chan error, cfg.WorkerCount)
	workers := make([]*consumer.Consumer, cfg.WorkerCount)
	for i := range cfg.WorkerCount {
		w, err := consumer.New(cfg.RabbitMQURL, cfg.RabbitMQExchange)
		if err != nil {
			log.Fatalf("FATAL worker %d: %v", i, err)
		}
		workers[i] = w
		go func(id int, c *consumer.Consumer) {
			log.Printf("INFO worker %d started", id)
			errCh <- c.Consume(proc.Handle)
		}(i, w)
	}
	defer func() {
		for _, w := range workers {
			w.Close()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		log.Printf("INFO received %s, shutting down", sig)
	case err := <-errCh:
		log.Printf("FATAL worker exited: %v", err)
		os.Exit(1)
	}
}
