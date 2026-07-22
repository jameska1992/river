# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

River is a self-hosted media platform composed of eight independent Go microservices in a monorepo, plus web / TV / Android client apps. Each service is a standalone Go module with its own `go.mod`. Each service also has its own `CLAUDE.md` with service-specific guidance — read it before working in a service.

## Common Commands

All services follow the same Go conventions:

```bash
go build ./...          # build
go test ./...           # run all tests
go test ./internal/...  # run tests for a specific package tree
go vet ./...            # static analysis
go mod tidy             # clean up dependencies
go run ./cmd/server     # run the service
```

There is no root-level Makefile or build script; run commands from within each service directory.

## Architecture

### Services and Data Flow

```
Filesystem
    └─→ [river-scan] ──→ RabbitMQ (river.media topic exchange)
                              ├─→ [river-video-trans]  → transcode + create DB records
                              ├─→ [river-audio-trans]  → transcode + create DB records
                              ├─→ [river-meta-movie]   → fetch TMDB metadata + update records
                              ├─→ [river-meta-tv]      → fetch TMDB metadata + update records
                              ├─→ [river-meta-book]    → fetch Open Library metadata + update records
                              └─→ [river-meta-music]   → fetch MusicBrainz metadata + update records

All services read/write to [river-api] via HTTP
         └─→ Clients (REST API)
```

### Services

| Service | Role |
|---|---|
| `river-api` | Central REST API (Gin + GORM/Postgres). All other services authenticate here and store records here. |
| `river-scan` | Walks the filesystem, hashes files for deduplication, publishes `media.discovered.*` events to RabbitMQ. |
| `river-video-trans` | Consumes movie/tvshow events, runs ffprobe/ffmpeg to H.264/AAC/MP4, creates media records via river-api. |
| `river-audio-trans` | Consumes music/audiobook events, transcodes to AAC .m4a, creates Artist/Album/Track and Audiobook/Chapter records via river-api. |
| `river-meta-movie` | Consumes RabbitMQ events, queries TMDB API, updates movie records in river-api. |
| `river-meta-tv` | Same as river-meta-movie but handles seasons and episodes (parses `SxxExx` patterns). |
| `river-meta-book` | Consumes `media.discovered.audiobook` events, queries Open Library API (no key required), updates audiobook records in river-api. |
| `river-meta-music` | Consumes `media.discovered.music` events, queries MusicBrainz + Cover Art Archive (no key required), updates music records in river-api. |

### river-api Layer Architecture

Strict top-down dependency (no layer imports a layer above it):

```
handlers → services → repository → database (GORM/Postgres)
                ↑
          apperrors  (zero dependencies — sentinel errors only)
```

- **`apperrors`**: defines `ErrNotFound`, `ErrConflict`, `ErrUnauthorized`
- **`repository`**: GORM structs and interfaces; each media type has its own file
- **`services`**: business logic; takes repository interfaces, returns `apperrors` sentinels on failure
- **`handlers`**: Gin handlers; call services and map errors to HTTP via `serviceStatus()` in `handlers/helpers.go`
- **`models`**: GORM models, all with UUID v4 PKs (auto-generated in `BeforeCreate`); array fields stored as JSON strings

### RabbitMQ

- Exchange: `river.media` (topic exchange)
- Routing keys: `media.discovered.movie`, `media.discovered.tvshow`, `media.discovered.music`, `media.discovered.audiobook`
- Queues: `river.video.trans` (movie+tvshow), `river.audio.trans` (music+audiobook), `river.meta.movie`, `river.meta.tvshow`, `river.meta.book` (audiobook), `river.meta.music` (music)

### Authentication (river-api)

- Access tokens: short-lived JWTs (HS256)
- Refresh tokens: opaque UUIDs stored in DB, rotated on use
- First registered user becomes `admin`; write operations require `admin` role

### Media Streaming (river-api)

Uses `http.ServeContent` for HTTP Range header support (enables seeking without full download).

## Environment Variables

**river-api**: `PORT`, `DATABASE_URL`, `JWT_SECRET`, `JWT_ACCESS_EXPIRY_MINUTES`, `JWT_REFRESH_EXPIRY_DAYS`, `MEDIA_BASE_PATH`

**river-scan**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `SCAN_INTERVAL`, `STATE_PATH`

**river-video-trans**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `WORKER_COUNT`, `OUTPUT_DIR`

**river-audio-trans**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `WORKER_COUNT`, `OUTPUT_DIR`

**river-meta-movie / river-meta-tv**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `TMDB_API_KEY`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `WORKER_COUNT`, `TMDB_IMAGE_BASE`

**river-meta-book**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `WORKER_COUNT`

## External Dependencies

- **RabbitMQ** — required by all services except river-api
- **FFmpeg/FFprobe** — system binaries required by river-video-trans and river-audio-trans
- **TMDB API** — API key required by river-meta-movie and river-meta-tv
- **Open Library** — used by river-meta-book; no API key required
