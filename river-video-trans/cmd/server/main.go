package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"river-video-trans/internal/apiclient"
	"river-video-trans/internal/config"
	"river-video-trans/internal/consumer"
	"river-video-trans/internal/processor"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL config: %v", err)
	}

	api := apiclient.New(cfg.RiverAPIURL, cfg.RiverAPIUsername, cfg.RiverAPIPassword, "river-video-trans")
	if err := api.Login(); err != nil {
		log.Fatalf("FATAL river-api login: %v", err)
	}
	log.Printf("INFO authenticated with river-api at %s", cfg.RiverAPIURL)
	api.Log("info", "started")

	// Use one connection for initial exchange/queue setup, then close it.
	setupCons, err := consumer.New(cfg.RabbitMQURL, cfg.RabbitMQExchange)
	if err != nil {
		log.Fatalf("FATAL rabbitmq setup: %v", err)
	}
	setupCons.Close()
	log.Printf("INFO exchange and queue declared, exchange=%s", cfg.RabbitMQExchange)

	proc := processor.New(api, cfg.OutputDir)

	// Each worker runs an auto-reconnect loop. A RabbitMQ blip, broker
	// restart, or transient network failure closes the delivery channel
	// and Consume returns; the worker reconnects with exponential backoff
	// instead of taking the whole container down (and forcing Docker to
	// restart it — which lost the in-flight state of every *other* worker
	// in the process). Only SIGTERM/SIGINT exits the process.
	for i := range cfg.WorkerCount {
		go runWorker(i, cfg.RabbitMQURL, cfg.RabbitMQExchange, proc.Handle)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("INFO received %s, shutting down", sig)
	api.Log("info", "shutting down")
}

// runWorker manages one consumer's lifecycle for the life of the process.
// It connects, runs Consume until it returns, closes the consumer, waits
// briefly, then reconnects. Backoff grows on repeated connect failures and
// resets after a successful connect.
func runWorker(id int, rabbitURL, exchange string, handler func(consumer.MediaDiscoveredEvent) error) {
	log.Printf("INFO worker %d started", id)
	const (
		minBackoff = time.Second
		maxBackoff = 30 * time.Second
	)
	backoff := minBackoff
	for {
		c, err := consumer.New(rabbitURL, exchange)
		if err != nil {
			log.Printf("WARN worker %d: rabbitmq connect failed: %v (retry in %s)", id, err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		// Successful connect — reset backoff so the next disconnect doesn't
		// inherit a stretched delay from a prior outage.
		backoff = minBackoff
		log.Printf("INFO worker %d connected to rabbitmq", id)

		consumeErr := c.Consume(handler)
		c.Close()
		if consumeErr != nil {
			log.Printf("WARN worker %d: consume returned err: %v (reconnecting)", id, consumeErr)
		} else {
			log.Printf("INFO worker %d: rabbitmq channel closed (reconnecting)", id)
		}
		// Brief pause before reconnect so we don't hammer the broker during
		// a recovery storm. Independent from the connect-failure backoff
		// above — this fires on clean channel closes too.
		time.Sleep(2 * time.Second)
	}
}
