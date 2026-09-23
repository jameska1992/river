// river-discord posts Discord notifications for River media events. It receives
// River's outbound webhooks (#195) — the lifecycle event envelope — verifies the
// HMAC signature, enriches from river-api, and posts a rich embed to a Discord
// channel (batching a burst of a title's episodes/tracks into one message).
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"river-discord/internal/apiclient"
	"river-discord/internal/config"
	"river-discord/internal/discord"
	"river-discord/internal/notifier"
	"river-discord/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL config: %v", err)
	}

	api := apiclient.New(cfg.RiverAPIURL, cfg.RiverAPIToken)
	note := notifier.New(api, discord.New(), cfg.BatchWindow, cfg.NotifyKinds, cfg.DiscordRoutes, cfg.DiscordWebhookURL)
	defer note.Close()

	srv := server.New(":"+cfg.Port, cfg.WebhookPath, cfg.WebhookSecret, note.Handle)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("INFO river-discord listening on :%s, webhook path %s (batch window %s)", cfg.Port, cfg.WebhookPath, cfg.BatchWindow)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		log.Printf("INFO received %s, shutting down", sig)
		_ = srv.Shutdown()
		note.Close()
	case err := <-errCh:
		log.Fatalf("FATAL server: %v", err)
	}
}
