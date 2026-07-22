# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./cmd/server   # Build the server binary
go vet ./...            # Run static analysis
go test ./...           # Run tests
go mod tidy             # Tidy dependencies
```

## Architecture

**river-video-trans** is a Go media transcoding service that bridges RabbitMQ message queues and a River API backend. It consumes media discovery events, optionally transcodes video files to H.264/AAC/MP4, and registers media metadata with the River API.

### Data flow

```
RabbitMQ (media.discovered.movie / media.discovered.tvshow)
  └── consumer.Consumer         # Parses JSON event, ACK/NACK
        └── processor.Handle()  # Routes by library type, parses filenames
              ├── transcoder.Probe() / Transcode()   # ffprobe + ffmpeg
              └── apiclient.Client                   # CRUD against River API
```

### Key packages

- **`cmd/server/main.go`** — Bootstraps config, authenticates with River API, declares RabbitMQ exchange/queue, spawns `WORKER_COUNT` independent consumer goroutines, handles graceful shutdown.
- **`internal/config`** — Reads environment variables. Required: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`. Optional with defaults: `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `RIVER_API_URL`, `OUTPUT_DIR`, `WORKER_COUNT` (default 2).
- **`internal/consumer`** — Each worker gets its own AMQP connection and channel with QoS prefetch=1. Binds to `river.video.trans` queue on exchange `river.media`. NACKs without requeue on any error.
- **`internal/processor`** — Core business logic. Parses directory/filename metadata using regex (`parseDirName`, `parseSeasonNumber`, `parseEpisodeNumber`, `parseEpisodeTitle`). Finds the largest file in the directory as the primary media file for movies.

  **Output layout** (`names.go`): canonical, title-driven (not source-tree mirroring). Movies land at `{OUTPUT_DIR}/movies/{Title} ({Year})/{Title}.mp4`; `({Year})` is omitted when year is 0 (unenriched record). TV episodes land at `{OUTPUT_DIR}/shows/{ShowName}/Season {N}/S{ss:02}E{ee:02}.mp4` — note the season directory name is unpadded (`Season 1`) while the filename is zero-padded (`S01E03.mp4`). Title/show name come from river-api (`movie.Title`, `tvshow.Title`), fetched up-front via `GetMovie`/`GetTVShow`, so admin renames via the "identify" flow propagate to disk on the next scan.

  `sanitizeFilename` replaces `\/:*?"<>|` with `-`, collapses hyphen/whitespace runs, trims edge dots/spaces/hyphens (Windows silently drops trailing dots), and falls back to `"untitled"` for empty input.

  When `OUTPUT_DIR` is empty, the transcoded file lands beside the source. Multiple libraries with the same movie title+year (or same show name) will collide at the canonical path — last write wins. The previous source-tree-mirroring layout (and any files written under it) are orphaned by this change and will be re-transcoded on first run.
- **`internal/apiclient`** — Thread-safe HTTP client. Handles automatic token re-authentication on 401. Paginates list endpoints at 200 items per page.
- **`internal/transcoder`** — Uses `ffprobe` to inspect streams. `NeedsTranscode()` returns true if codec is not H.264, audio is not AAC, container is not MP4, or resolution exceeds 1080p. Transcodes with CRF 23, preset medium; copies streams that already match the target.

### Worker concurrency model

Each of the `WORKER_COUNT` workers is an independent goroutine with its own RabbitMQ connection. RabbitMQ round-robins messages across workers. A worker crash propagates a fatal error to main, which exits the entire process.

### System dependencies

`ffmpeg` and `ffprobe` must be installed and on `PATH`.
