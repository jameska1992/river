package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"time"

	"river-events/internal/apiclient"
	"river-events/internal/config"
	"river-events/internal/consumer"
	"river-events/internal/events"
	"river-events/internal/processor"
	"river-events/internal/webhooks"
)

// river-events joins media.transcoded.* and media.enriched.* into
// media.ready.* on the river.lifecycle exchange. It reads media records back
// from river-api to decide readiness, so it holds no state of its own.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL config: %v", err)
	}

	api := apiclient.New(cfg.RiverAPIURL, cfg.RiverAPIUsername, cfg.RiverAPIPassword, cfg.RiverAPIKey, "river-events")
	if err := api.Login(); err != nil {
		log.Fatalf("FATAL river-api login: %v", err)
	}
	log.Printf("INFO authenticated with river-api at %s", cfg.RiverAPIURL)
	api.Log("info", "started")

	pub, err := events.NewPublisher(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("FATAL rabbitmq publisher: %v", err)
	}
	defer pub.Close()

	// Webhook fan-out: cache the active-webhook list (refreshed from river-api)
	// and a publisher onto the per-delivery queue.
	hookCache := webhooks.NewCache(api.GetActiveWebhooks, 30*time.Second)
	deliveryPub, err := webhooks.NewPublisher(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("FATAL webhook delivery publisher: %v", err)
	}
	defer deliveryPub.Close()

	proc := processor.New(api, pub, hookCache, deliveryPub)

	// Delivery workers: consume the per-delivery queue, HMAC-sign + POST each,
	// and report the outcome back to river-api for admin observability.
	report := func(d webhooks.Delivery, status string, attempts, code int, errMsg string) {
		_ = api.RecordDelivery(apiclient.DeliveryOutcome{
			WebhookID: d.WebhookID, Event: d.Event.RoutingKey(), MediaID: d.Event.MediaID,
			Status: status, Attempts: attempts, ResponseCode: code, Error: errMsg,
		})
	}
	for i := 0; i < cfg.WorkerCount; i++ {
		dc, err := webhooks.NewConsumer(cfg.RabbitMQURL, cfg.MaxRetries, cfg.RetryBackoff, report)
		if err != nil {
			log.Fatalf("FATAL webhook delivery worker %d: %v", i, err)
		}
		go func(id int, c *webhooks.Consumer) {
			log.Printf("INFO webhook delivery worker %d started", id)
			if err := c.Consume(); err != nil {
				log.Printf("ERROR webhook delivery worker %d: %v", id, err)
			}
		}(i, dc)
	}

	// Declare topology up-front so the exchange/queues exist before workers.
	setup, err := consumer.New(cfg.RabbitMQURL, cfg.MaxRetries, cfg.RetryBackoff, nil)
	if err != nil {
		log.Fatalf("FATAL rabbitmq setup: %v", err)
	}
	setup.Close()
	log.Printf("INFO lifecycle exchange %q and queue declared", events.Exchange)

	errCh := make(chan error, cfg.WorkerCount)
	workers := make([]*consumer.Consumer, cfg.WorkerCount)
	for i := range cfg.WorkerCount {
		w, err := consumer.New(cfg.RabbitMQURL, cfg.MaxRetries, cfg.RetryBackoff, nil)
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
