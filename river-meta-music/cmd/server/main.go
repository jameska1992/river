package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"river-meta-music/internal/apiclient"
	"river-meta-music/internal/config"
	"river-meta-music/internal/consumer"
	"river-meta-music/internal/musicbrainz"
	"river-meta-music/internal/processor"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL config: %v", err)
	}

	api := apiclient.New(cfg.RiverAPIURL, cfg.RiverAPIUsername, cfg.RiverAPIPassword, "river-meta-music")
	if err := api.Login(); err != nil {
		log.Fatalf("FATAL river-api login: %v", err)
	}
	log.Printf("INFO authenticated with river-api at %s", cfg.RiverAPIURL)
	api.Log("info", "started")

	mb := musicbrainz.New()

	// Declare exchange and queue, then close the setup connection.
	setupCons, err := consumer.New(cfg.RabbitMQURL, cfg.RabbitMQExchange)
	if err != nil {
		log.Fatalf("FATAL rabbitmq setup: %v", err)
	}
	setupCons.Close()
	log.Printf("INFO exchange and queue declared, exchange=%s", cfg.RabbitMQExchange)

	proc := processor.New(api, mb)

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /refresh/artist/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if err := proc.RefreshByArtistID(id); err != nil {
				log.Printf("ERROR refresh artist %s: %v", id, err)
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
