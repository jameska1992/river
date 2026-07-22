# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...          # compile
go vet ./...            # lint
go test ./...           # test
go run ./cmd/server     # run locally (requires env vars below)
```

Required environment variables to run:
```
RIVER_API_USERNAME=...
RIVER_API_PASSWORD=...
TMDB_API_KEY=...
```

Optional (defaults shown):
```
RIVER_API_URL=http://localhost:8080
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=river.media
WORKER_COUNT=2
TMDB_IMAGE_BASE=https://image.tmdb.org/t/p/original
```

## Architecture

This service enriches TV show records in river-api with metadata from TMDB. It is a sibling of `../river-meta-movie`; both follow the same structure.

**Event flow:** river-scanner publishes `MediaDiscoveredEvent` messages to RabbitMQ exchange `river.media` with routing key `media.discovered.tvshow`. This service consumes them from durable queue `river.meta.tvshow`.

**One event = one season.** The scanner emits a separate event per season directory, with `directory_name` (show folder name), `season_name` (e.g. `"Season 1"`), `season_path` (full path to season folder), and `files` (full paths to video files within it).

**Processing per event (`processor.Handle`):**
1. Find the matching `TVShow` record in river-api by title — if absent, ACK and skip (river-trans creates it on transcode).
2. `PUT /api/tvshows/{id}` with TMDB show metadata (title, description, year, status, genres, rating, poster, backdrop). This always overwrites.
3. Parse the season number from `season_name` (first integer found). List existing seasons; create the season via `POST /api/tvshows/{id}/seasons` only if that number is absent — river-api has no season update endpoint.
4. For each file, parse the episode number (SxxExx → Ex fallback regex). List existing episodes; create missing ones via `POST .../episodes` using TMDB episode metadata. Episodes are also create-only.

**Package responsibilities:**
- `internal/consumer` — AMQP wiring (exchange declare, queue bind, QoS prefetch=1, ack/nack)
- `internal/tmdb` — three TMDB calls: `/3/search/tv`, `/3/tv/{id}`, `/3/tv/{id}/season/{n}`
- `internal/apiclient` — river-api HTTP client with Bearer auth and automatic re-login on 401
- `internal/processor` — all business logic; the other packages are pure I/O with no decisions
- `internal/config` — env var loading; missing required vars fail at startup

**Concurrency:** `WORKER_COUNT` goroutines each hold an independent AMQP connection+channel. RabbitMQ round-robins deliveries across them.

**Genres** are stored as a JSON-encoded string (`"[\"Drama\",\"Crime\"]"`), not a native JSON column — match this when reading or writing the field.
