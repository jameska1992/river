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

## Releasing

The repo is versioned with a single top-level `VERSION` file (semver `X.Y.Z`, e.g. `0.3.0`) — the single source of truth. Releases are marked by an annotated `v<VERSION>` tag and a long-lived `release-v<VERSION>` branch, all pointing at the same commit on `main`.

CI (`.github/workflows/ci.yml`) has a `version` job that validates `VERSION` is semver on every build, and on a tag push (`refs/tags/v*`) enforces that the tag equals `v<VERSION>` — so the tag and the file can never drift.

To cut a release (all from `main`, once it's green):

```bash
# 1. Bump the version (edit VERSION, e.g. 0.3.0 → 0.4.0), commit via PR, merge to main.
# 2. From the merge commit on main:
git tag -a v0.4.0 -m "Release v0.4.0 …"   # annotated; summarize notable PRs in the message
git branch release-v0.4.0                  # cut the release branch from the same commit
git push origin v0.4.0                      # tag push → CI verifies tag == v<VERSION>
git push -u origin release-v0.4.0
```

Conventions: tags are `v0.3.0` (with `v` prefix); release branches are `release-v0.3.0` (with `v` prefix); the `VERSION` file itself holds the bare `0.3.0` (no `v`). `river-api` deliberately has no RabbitMQ dependency, but every other service does — keep shared dependency bumps (e.g. `amqp091-go`) in sync across all affected `go.mod` files in one release.

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
- **Retry + dead-letter:** each consumer also declares `<queue>.retry` and `<queue>.dlq`. On a handler error the message is republished to `<queue>.retry` (a consumer-less queue with a per-message TTL that dead-letters back to the work queue after `RETRY_BACKOFF`), carrying an `x-retry-count` header. After `MAX_RETRIES` attempts it's parked in `<queue>.dlq` (with an `x-death-reason` header) instead of being dropped or looping forever. The work queue's own declaration is left unchanged, so this upgrades cleanly on installs where the queue already exists.

### Authentication (river-api)

- Access tokens: short-lived JWTs (HS256)
- Refresh tokens: opaque UUIDs stored in DB, rotated on use
- First registered user becomes `admin` (the bootstrap keys off "no admin exists yet", so a seeded service account doesn't consume it)
- Roles: `admin` (full), `user` (read), `service` (least-privilege, non-human). Media create/update endpoints require `admin` or `service` (`AdminOrService`); the destructive/admin surface (user management, deletes, settings writes, scan control) stays `admin`-only. The `service` role is provisioned only by boot-time seeding (`RIVER_SERVICE_USERNAME`/`RIVER_SERVICE_PASSWORD`, create-if-absent) — not assignable via the API — and is what the internal services authenticate with

### Media Streaming (river-api)

Uses `http.ServeContent` for HTTP Range header support (enables seeking without full download).

## Environment Variables

**river-api**: `PORT`, `DATABASE_URL`, `JWT_SECRET`, `JWT_ACCESS_EXPIRY_MINUTES`, `JWT_REFRESH_EXPIRY_DAYS`, `MEDIA_BASE_PATH`, `RIVER_SERVICE_USERNAME`, `RIVER_SERVICE_PASSWORD`

**river-scan**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `SCAN_INTERVAL`, `STATE_PATH`

**river-video-trans**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `WORKER_COUNT`, `OUTPUT_DIR`, `MAX_RETRIES`, `RETRY_BACKOFF`

**river-audio-trans**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `WORKER_COUNT`, `OUTPUT_DIR`, `MAX_RETRIES`, `RETRY_BACKOFF`

**river-meta-movie / river-meta-tv**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `TMDB_API_KEY`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `WORKER_COUNT`, `TMDB_IMAGE_BASE`, `MAX_RETRIES`, `RETRY_BACKOFF`

**river-meta-book**: `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RIVER_API_URL`, `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `WORKER_COUNT`, `MAX_RETRIES`, `RETRY_BACKOFF`

## External Dependencies

- **RabbitMQ** — required by all services except river-api
- **FFmpeg/FFprobe** — system binaries required by river-video-trans and river-audio-trans
- **TMDB API** — API key required by river-meta-movie and river-meta-tv
- **Open Library** — used by river-meta-book; no API key required
