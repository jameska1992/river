package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"river-scan/internal/apiclient"
	"river-scan/internal/config"
	"river-scan/internal/publisher"
	"river-scan/internal/scanner"
	"river-scan/internal/state"

	"github.com/google/uuid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := state.Load(cfg.StatePath)
	if err != nil {
		log.Fatalf("load state: %v", err)
	}

	var pub *publisher.Publisher
	if !cfg.DisableTranscoding {
		pub, err = publisher.New(cfg.RabbitMQURL, cfg.RabbitMQExchange)
		if err != nil {
			log.Fatalf("publisher: %v", err)
		}
		defer pub.Close()
	} else {
		log.Println("transcoding disabled: registering media files directly")
	}

	api := apiclient.New(cfg.APIBaseURL, cfg.APIUsername, cfg.APIPassword, "river-scan")
	s := scanner.New(api, pub, st, cfg.DisableTranscoding, cfg.MaxScanDepth)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Buffered channel size 1 — a second trigger while one is queued is silently dropped.
	triggerCh := make(chan struct{}, 1)

	// HTTP server for manual trigger.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /trigger", func(w http.ResponseWriter, r *http.Request) {
		select {
		case triggerCh <- struct{}{}:
		default: // already queued, ignore
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, r *http.Request) {
		// Used by the admin "Scanner State" page in river-web (proxied via
		// river-api) to list the content-hash + show-id maps so an admin
		// can remove individual entries and force a rescan. The state file
		// itself isn't readable from the API container — this endpoint is
		// the only way to inspect it.
		dirs, shows := st.Snapshot()
		body := struct {
			Directories map[string]state.DirectoryRecord `json:"directories"`
			Shows       map[string]string                `json:"shows"`
		}{dirs, shows}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(body); err != nil {
			log.Printf("state: encode response: %v", err)
		}
	})
	mux.HandleFunc("POST /forget", func(w http.ResponseWriter, r *http.Request) {
		// Called by river-api when an admin deletes a media row, so the
		// next scan doesn't see the content hash as "already known" and
		// silently skip re-discovery. `paths` are exact Directories keys
		// (movie source files, season directories, etc.); `prefixes` are
		// parent directories whose entire subtree should be forgotten
		// (used by TV/audiobook delete, where some children may not be
		// enumerable from the DB because metadata enrichment left them
		// empty); `shows` are folder-path → show-id mappings cached for
		// TV resolution.
		var req struct {
			Paths    []string `json:"paths"`
			Shows    []string `json:"shows"`
			Prefixes []string `json:"prefixes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		for _, p := range req.Paths {
			st.Forget(p)
		}
		for _, p := range req.Prefixes {
			st.ForgetPrefix(p)
		}
		for _, p := range req.Shows {
			st.ForgetShow(p)
		}
		if err := st.Flush(); err != nil {
			log.Printf("forget: flush state: %v", err)
			http.Error(w, "flush state", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /requeue-untranscoded", func(w http.ResponseWriter, r *http.Request) {
		// Run async so the river-api proxy doesn't time out on large
		// libraries. Counts are logged + sent through api.Log on completion;
		// progress can also be eyeballed via the RabbitMQ management UI.
		go func() {
			counts, err := s.RequeueUntranscoded(context.Background())
			if err != nil {
				log.Printf("requeue-untranscoded error: %v", err)
				api.Log("error", "requeue-untranscoded: "+err.Error())
				return
			}
			log.Printf("requeue-untranscoded complete: %+v", counts)
		}()
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /scan-dir", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LibraryID   string `json:"library_id"`
			LibraryType string `json:"library_type"`
			DirPath     string `json:"dir_path"`
			ShowPath    string `json:"show_path"`
			ShowName    string `json:"show_name"`
			Force       bool   `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LibraryID == "" || req.DirPath == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		lib := apiclient.Library{ID: req.LibraryID, Type: req.LibraryType}
		go func() {
			if err := s.ScanDir(context.Background(), lib, req.DirPath, req.ShowPath, req.ShowName, req.Force); err != nil {
				log.Printf("scan-dir error: %v", err)
			}
		}()
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /retranscode", func(w http.ResponseWriter, r *http.Request) {
		// Publish a single force-transcode event for one already-discovered
		// movie or episode. river-api builds the payload from the media record
		// (it holds the ids + source path) and proxies here because it has no
		// RabbitMQ connection of its own. Unlike a scan, this bypasses the
		// content-hash state entirely — the event is emitted verbatim with
		// force_transcode set so the transcoder overwrites the existing output.
		if pub == nil {
			http.Error(w, "transcoding is disabled", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			LibraryID   string `json:"library_id"`
			LibraryType string `json:"library_type"` // "movie" | "tvshow"
			MediaID     string `json:"media_id"`     // movie id, or show id for episodes
			SeasonID    string `json:"season_id"`    // episodes only
			SeasonName  string `json:"season_name"`  // "Season N" — episodes only
			File        string `json:"file"`         // absolute source path to re-transcode
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
			req.LibraryID == "" || req.MediaID == "" || req.File == "" ||
			(req.LibraryType != "movie" && req.LibraryType != "tvshow") {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		event := publisher.MediaDiscoveredEvent{
			EventID:        uuid.New().String(),
			LibraryID:      req.LibraryID,
			LibraryType:    req.LibraryType,
			DirectoryName:  filepath.Base(filepath.Dir(req.File)),
			DirectoryPath:  filepath.Dir(req.File),
			SeasonName:     req.SeasonName,
			MediaID:        req.MediaID,
			SeasonID:       req.SeasonID,
			Files:          []string{req.File},
			DiscoveredAt:   time.Now().UTC(),
			ForceTranscode: true,
		}
		if err := pub.Publish(context.Background(), event); err != nil {
			log.Printf("retranscode publish error: %v", err)
			api.Log("error", "retranscode: "+err.Error())
			http.Error(w, "publish failed", http.StatusBadGateway)
			return
		}
		log.Printf("retranscode queued: type=%s media=%s file=%q", req.LibraryType, req.MediaID, req.File)
		w.WriteHeader(http.StatusAccepted)
	})
	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: mux}
	go func() {
		log.Printf("scan trigger server listening on :%s", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("trigger server error: %v", err)
		}
	}()
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	}()

	// The scan interval lives in river-api's settings store. An unset
	// interval means "single scan and exit" (matches the historical
	// SCAN_INTERVAL-unset behaviour); a set interval runs the scan loop.
	interval, err := api.ScanInterval()
	if err != nil {
		log.Printf("WARN could not fetch scan interval, defaulting to %s: %v", defaultScanInterval, err)
		interval = defaultScanInterval
	}
	if interval == 0 {
		if err := s.Run(ctx); err != nil {
			log.Fatalf("scan: %v", err)
		}
		return
	}

	api.Log("info", "started")
	log.Printf("starting scanner (interval: %s)", interval)
	for {
		if err := s.Run(ctx); err != nil {
			api.Log("error", "scan error: "+err.Error())
			log.Printf("scan error: %v", err)
		}
		// Re-read the interval each cycle so an admin's change (via the
		// settings UI) takes effect on the next tick without a restart.
		// Keep the last-known value if the lookup fails or is cleared.
		if d, err := api.ScanInterval(); err == nil && d > 0 {
			interval = d
		}
		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return
		case <-triggerCh:
			log.Println("manual scan triggered")
		case <-time.After(interval):
		}
	}
}

const defaultScanInterval = time.Hour
