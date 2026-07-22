# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...          # build all packages
go vet ./...            # static analysis
go mod tidy             # sync go.mod/go.sum after dependency changes
go run ./cmd/server     # run locally
```

## Architecture

river-meta-movie is a RabbitMQ consumer that enriches movie records in river-api with metadata from TMDB. It is one of four services in the River media platform:

- **river-scan** — scans the filesystem and publishes `MediaDiscoveredEvent` messages to the `river.media` topic exchange
- **river-trans** — consumes the same events, transcodes video files, and creates/updates movie records in river-api
- **river-meta-movie** (this service) — consumes the same events and enriches the movie records with TMDB metadata
- **river-api** — the central REST API backed by Postgres/GORM that all services read and write

### Event flow

```
river-scan → RabbitMQ (exchange: river.media, key: media.discovered.movie)
                ├─→ river-trans  (queue: river.trans)
                └─→ river-meta-movie  (queue: river.meta.movie)
```

Both river-trans and river-meta-movie receive every movie event independently. Because river-trans creates the movie record and river-meta-movie enriches it, there is a race: if the enrichment event arrives before river-trans has written the record, `processor.Handle` logs a warning and returns `nil` (ACK, no requeue). The scanner re-publishes events on its next interval, which retries the enrichment.

### Package responsibilities

- `internal/config` — loads and validates environment variables
- `internal/apiclient` — HTTP client for river-api; handles JWT auth, auto re-login on 401, and paginated `ListMovies`
- `internal/consumer` — wraps amqp091-go; declares the exchange/queue/binding, sets QoS prefetch=1, ACKs on success and NACKs (no requeue) on error
- `internal/tmdb` — TMDB API v3 client; searches by title+year, falls back to title-only if no results, fetches full details
- `internal/processor` — ties everything together: parses `"Title (YYYY)"` from `DirectoryName`, matches the API record by title, fetches TMDB metadata, marshals genres as a JSON string, and calls `UpdateMovie`

### Key conventions (shared with river-trans/river-scan)

- **Worker model**: `main.go` spins up `WORKER_COUNT` goroutines, each with its own AMQP connection. RabbitMQ round-robins messages across them.
- **Genres storage**: the `genres` field in river-api is a JSON-encoded string (e.g. `"[\"Action\",\"Drama\"]"`), not a native JSON column. Always `json.Marshal([]string{...})` before setting it.
- **Authentication**: services authenticate with river-api using username/password at startup and on every 401 response (`doWithRetry` in apiclient).
- **Pagination**: `paginateAll` fetches pages of 200 until a page returns fewer than 200 items.

## Environment variables

| Variable | Default | Required |
|---|---|---|
| `RIVER_API_USERNAME` | — | yes |
| `RIVER_API_PASSWORD` | — | yes |
| `TMDB_API_KEY` | — | yes |
| `RIVER_API_URL` | `http://localhost:8080` | no |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | no |
| `RABBITMQ_EXCHANGE` | `river.media` | no |
| `WORKER_COUNT` | `2` | no |
| `TMDB_IMAGE_BASE` | `https://image.tmdb.org/t/p/original` | no |
