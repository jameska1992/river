# River

![River](./docs/assets/river-logo-small.png)

Self-hosted media platform. Movies, TV shows, music, and audiobooks — scanned from disk, transcoded to browser-friendly formats, enriched with metadata, streamed to a web browser or a TV app.

## Features

**Media library**
- Four media types, end to end: **movies**, **TV shows** (seasons + episodes), **music** (artists / albums / tracks), and **audiobooks** (with chapters).
- Automatic filesystem scanning with incremental rescans — a size + mod-time hash short-circuits unchanged files — and `SxxExx` season/episode parsing.
- Metadata enrichment from **TMDB** (movies + TV), **Open Library** (audiobooks), and **MusicBrainz** + **Cover Art Archive** + **Wikipedia** (music) — posters, backdrops, descriptions, cast/crew, ratings, and genres.
- Pre-transcoded libraries can skip the transcode step and stream as-is.

**Playback & streaming**
- Media is transcoded once at ingestion to browser-friendly **H.264 / AAC / MP4** (video) and **AAC / M4A** (audio), capped at 1080p — playback streams the pre-transcoded files directly, no per-request transcoding.
- Optional **NVENC GPU acceleration** with automatic fallback to CPU (`libx264`).
- **HTTP Range** streaming for instant seeking, plus **WebVTT subtitle** extraction and **multiple audio tracks**.
- **Continue watching**, per-title watch progress, "next up" suggestions, and optional file **downloads**.
- **Watch party** — real-time synchronized playback across viewers over WebSockets.

**Discovery**
- Home page with hero banner, continue-watching, and recently-added rows.
- Browse grids per library, global search, cast/crew credits, and "similar titles".
- User **collections** and **watchlist**.

**Requests — Radarr & Sonarr integration**
- Search **Radarr** (movies) and **Sonarr** (TV) for titles you don't own yet, right from the web client.
- Request a title and River adds it to Radarr/Sonarr for you — root folder and quality profile are auto-selected from each server's defaults, so there's nothing to configure per request.
- A combined **calendar** merges upcoming and recently-released dates from both Radarr and Sonarr into one upcoming-releases view.
- Fully optional: point River at your `*arr` instances with URL + API key, or leave it off (request endpoints simply report unavailable when unconfigured).

**Clients**
- **`river-web`** — full browser client and admin surface.
- **`river-tv`** — TV-optimized client with D-pad / spatial focus navigation.
- **`river-tv-android`** — Android TV / Fire TV launcher app wrapping the TV client.

**Accounts & administration**
- JWT auth with rotating refresh tokens; the first registered user becomes **admin**.
- Admin dashboard: library stats, library management, media upload, unidentified items, scanner state, users, active sessions, and service logs.

**Platform**
- Eight independent Go microservices, decoupled via **RabbitMQ**.
- One-command **Docker Compose** deployment (with an opt-in NVIDIA GPU overlay) and env-based configuration.
- **Swagger / OpenAPI** docs, paginated list endpoints, and an image proxy for CDN artwork.

## Screenshots

| | |
|---|---|
| ![Home page](./docs/assets/homepage01.png) | ![Browse library](./docs/assets/homepage02.png) |
| Home page — hero banner and content rows | Browse — poster grid across libraries |
| ![Detail page](./docs/assets/details01.png) | ![Admin overview](./docs/assets/admin01.png) |
| Detail page — synopsis, cast, and trailer | Admin — library stats and overview |

![Service logs](./docs/assets/logs01.png)

*Service logs — metadata enrichment identifying titles via TMDB.*

## Architecture at a glance

```
Filesystem
    └─→ [river-scan] ──→ RabbitMQ (river.media topic exchange)
                              ├─→ [river-video-trans]  H.264/AAC/MP4 (+NVENC opt.)
                              ├─→ [river-audio-trans]  AAC .m4a
                              ├─→ [river-meta-movie]   TMDB enrichment
                              ├─→ [river-meta-tv]      TMDB enrichment (seasons/eps)
                              ├─→ [river-meta-book]    Open Library
                              └─→ [river-meta-music]   MusicBrainz

              [river-api]  ← records ← every producer above
                    ↓
    ┌──────────────┼──────────────────┐
[river-web]   [river-tv]      [river-tv-android]
 (browser)     (Vite web)       (Fire/Android TV)
```

- **`river-api`** owns Postgres + auth + streaming. Every other backend service is an HTTP client to it.
- **RabbitMQ** is the only inter-service coupling on the ingest side. Every downstream consumer listens for `media.discovered.*` events and works independently.
- **Client apps** all talk to `river-api` and are otherwise independent.

## Repo layout

**Backend services** (Go, one module each):

| Service | Role |
|---|---|
| [`river-api`](./river-api/) | Central REST API (Gin + GORM + Postgres). Auth, media CRUD, streaming, image proxy. |
| [`river-scan`](./river-scan/) | Filesystem walker. Publishes discovery events to RabbitMQ. |
| [`river-video-trans`](./river-video-trans/) | Transcodes movies + TV episodes. NVENC where available, libx264 fallback. |
| [`river-audio-trans`](./river-audio-trans/) | Transcodes music + audiobooks to AAC/M4A. |
| [`river-meta-movie`](./river-meta-movie/) | TMDB enrichment for movies. |
| [`river-meta-tv`](./river-meta-tv/) | TMDB enrichment for shows / seasons / episodes. |
| [`river-meta-book`](./river-meta-book/) | Open Library enrichment for audiobooks. |
| [`river-meta-music`](./river-meta-music/) | MusicBrainz enrichment for music. |

**Client apps**:

| Client | Stack |
|---|---|
| [`river-web`](./river-web/) | Browser client — React + Vite. Also serves as the admin surface. |
| [`river-tv`](./river-tv/) | TV-optimised web app — React + Vite. D-pad focus navigation. |
| [`river-tv-android`](./river-tv-android/) | Android TV / Fire TV launcher app — WebView-wraps the `river-tv` build. |

## Deployment

There are two ways to run River: **with Docker Compose** (recommended — one command brings up every service, Postgres, and RabbitMQ) or **without Docker**, running the services from source against a Postgres and RabbitMQ you provide.

### Deploy with Docker (recommended)

The entire backend **and** the web client run from Docker Compose. Every service — the Go microservices, RabbitMQ, Postgres, and the nginx-served web app — builds and runs inside containers, so the host only needs Docker itself.

#### Prerequisites

| Requirement | Needed for | Notes |
|---|---|---|
| **Docker Engine 24+** and the **Compose v2** plugin | Everything | The only hard requirement. Images build in-container — no local Go or Node toolchain needed to run the stack. |
| **TMDB API key** (free) | Movie & TV metadata | Get one at [themoviedb.org/settings/api](https://www.themoviedb.org/settings/api). Required by `river-meta-movie` / `river-meta-tv`. |
| **NVIDIA GPU + drivers + [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)** | *Optional* — NVENC hardware transcoding | Only when using the GPU overlay (below). Video transcoding falls back to CPU `libx264` without it. |

#### 1. Configure

```bash
cp .env.example .env
```

Edit `.env` and set the **required** values:

| Variable | Description |
|---|---|
| `MEDIA_PATH` | Absolute host path to your media library (mounted read-only into the services). |
| `OUTPUT_PATH` | Absolute host path where transcoded output is written. |
| `JWT_SECRET` | Long random string used to sign access tokens. |
| `ADMIN_PASSWORD` | Password for the admin account created on first boot. |
| `TMDB_API_KEY` | Your TMDB key (see prerequisites). |

Everything else has sensible defaults — Postgres/RabbitMQ credentials, admin username/email, `SCAN_INTERVAL`, ffmpeg worker counts, JWT token lifetimes, and the optional Radarr/Sonarr integration. See [`.env.example`](./.env.example) for the full list.

#### 2. Start the stack

```bash
# CPU-only (default)
docker compose up -d --build

# With NVENC on an NVIDIA GPU (requires the NVIDIA Container Toolkit)
docker compose -f docker-compose.yml -f docker-compose.gpu.yml up -d --build
```

On first boot a one-shot `river-init` container registers the admin user from `ADMIN_USERNAME` / `ADMIN_PASSWORD`, then the media services come up and `river-scan` begins indexing `MEDIA_PATH`.

#### Updating & teardown

```bash
docker compose up -d --build      # rebuild & roll out after pulling changes
docker compose down               # stop and remove containers (keeps volumes/data)
docker compose down -v            # also wipe Postgres, RabbitMQ, and scan state
```

### Deploy without Docker (from source)

Run the services directly with the Go toolchain. You provide Postgres, RabbitMQ, and ffmpeg; each service is a standalone Go module configured entirely through environment variables.

#### Prerequisites

| Requirement | Needed for |
|---|---|
| **Go 1.26+** | Building/running all eight backend services (matches the `go` directive in each `go.mod`). |
| **PostgreSQL 16** | `river-api`'s datastore. |
| **RabbitMQ 3+** (with the topic exchange enabled) | Inter-service messaging for every service except river-api. |
| **FFmpeg / FFprobe** on `PATH` | `river-video-trans` and `river-audio-trans` transcoding. |
| **Node.js 20+** | Building the web client. |
| **TMDB API key** (free) | `river-meta-movie` / `river-meta-tv`. |
| **NVIDIA drivers + NVENC-enabled ffmpeg** | *Optional* hardware transcoding; CPU `libx264` is used otherwise. |

#### 1. Provision Postgres & RabbitMQ

Create a database and a RabbitMQ user, then note the connection strings. river-api defaults to `postgres://river:river@localhost:5432/river?sslmode=disable`; the workers default to a local RabbitMQ. Override via the env vars below if yours differ.

#### 2. Start river-api

```bash
cd river-api
DATABASE_URL='postgres://river:river@localhost:5432/river?sslmode=disable' \
JWT_SECRET='<long-random-string>' \
MEDIA_BASE_PATH='/srv/media' \
PORT=8080 \
go run ./cmd/server
```

river-api migrates its schema on startup. Register the admin account (the first user is promoted to admin automatically):

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","email":"admin@river.local","password":"<password>"}'
```

#### 3. Start the worker services

In separate shells, run each service with its own environment. All share `RIVER_API_URL`, `RIVER_API_USERNAME`, `RIVER_API_PASSWORD`, `RABBITMQ_URL`, and `RABBITMQ_EXCHANGE=river.media`; the transcoders also need `OUTPUT_DIR`, and the metadata services need their API keys. Each service's exact variables are listed in its own `README.md` / `CLAUDE.md`.

```bash
cd river-scan        && go run ./cmd/server   # filesystem scanner
cd river-video-trans && go run ./cmd/server   # movie/TV transcoder (needs ffmpeg)
cd river-audio-trans && go run ./cmd/server   # music/audiobook transcoder (needs ffmpeg)
cd river-meta-movie  && go run ./cmd/server   # TMDB movie metadata (needs TMDB_API_KEY)
cd river-meta-tv     && go run ./cmd/server   # TMDB TV metadata (needs TMDB_API_KEY)
cd river-meta-book   && go run ./cmd/server   # Open Library metadata
cd river-meta-music  && go run ./cmd/server   # MusicBrainz metadata
```

Point river-api's `RIVER_SCAN_URL` / `RIVER_META_*_URL` variables at wherever each service listens so the admin "refresh metadata" / "scan now" actions can reach them.

#### 4. Build & serve the web client

```bash
cd river-web
npm ci
npm run build          # outputs static files to river-web/dist
```

Serve `river-web/dist` with any static web server, proxying `/api` (and `/health`) to river-api — see [`river-web/nginx.conf`](./river-web/nginx.conf) for a reference reverse-proxy config. For local development, `npm run dev` runs a Vite dev server (override the API target with `RIVER_API_TARGET`).

### Accessing River

| Service | URL | Notes |
|---|---|---|
| Web client | `http://<host>/` | Serves the SPA and proxies `/api` to river-api (incl. WebSocket + media streaming). Port `80` under Docker; whatever you bind when self-hosting. |
| REST API | `http://<host>:8080/api` | Exposed directly as well as via the web proxy. |
| API docs (Swagger) | `http://<host>:8080/swagger/index.html` | Interactive OpenAPI explorer. |
| RabbitMQ management | `http://<host>:15672` | Login with your RabbitMQ credentials. |

> **Production note:** the Docker compose file publishes Postgres (`5432`) and RabbitMQ (`5672`/`15672`) to the host for convenience — remove those `ports` mappings when deploying to an untrusted network.

### Adding media

Drop files under your media path using the layout described in [`river-scan`](./river-scan/) (e.g. `Movies/Title (Year)/…`, `Shows/Show/Season 01/Show - S01E01.…`). `river-scan` rescans every `SCAN_INTERVAL`, or trigger a scan immediately from the admin dashboard (**Scan Now**). Transcoded, streamable copies land in the output path.

### TV & Android clients

`river-web` is covered above. The **`river-tv`** (TV-optimised web) and **`river-tv-android`** (Fire TV / Android TV) clients are built and deployed separately — see each client's own README. Building the Android app additionally needs **JDK 17** and the **Android SDK** (compileSdk 35).

## Documentation

Each service directory has a `CLAUDE.md` (originally written to guide Claude Code) that doubles as authoritative architecture reference — layer boundaries, patterns, tradeoffs. Start there when working on that specific service.

## Environment variables

Full list of required and optional env vars lives in [`.env.example`](./.env.example). Each service also documents its own subset in its README.

## License

River is licensed under the [GNU Affero General Public License v3.0](./LICENSE) (AGPL-3.0).
