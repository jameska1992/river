# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...      # build
go vet ./...        # static analysis
go mod tidy         # sync dependencies
go run ./cmd/server # run locally
```

Required environment variables:
```
RIVER_API_USERNAME=...
RIVER_API_PASSWORD=...
```

Optional (defaults shown):
```
RIVER_API_URL=http://localhost:8080
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=river.media
WORKER_COUNT=2
PORT=8084
```

## Architecture

river-meta-music enriches artist and album records in river-api with metadata from MusicBrainz (musicbrainz.org). No API key required. Album cover art comes from the Cover Art Archive (coverartarchive.org). Artist bios are fetched from Wikipedia via MusicBrainz URL relations.

**Event flow:**
```
river-scan → RabbitMQ (exchange: river.media, key: media.discovered.music)
                ├─→ river-audio-trans   (queue: river.audio.trans)
                └─→ river-meta-music    (queue: river.meta.music)
```

Both consumers receive every music event independently. river-audio-trans creates the Artist/Album/Track records; river-meta-music enriches them. If the artist record is not found via `MediaID`, the processor logs a warning and ACKs (the scanner will re-publish on the next interval).

### Package responsibilities

- `internal/config` — env var loading; missing required vars fail at startup
- `internal/consumer` — AMQP wiring; queue `river.meta.music` binds `media.discovered.music`; QoS prefetch=1; ACK on success, NACK (no requeue) on error
- `internal/apiclient` — river-api HTTP client; Bearer auth with automatic re-login on 401; Artist + Album CRUD
- `internal/musicbrainz` — MusicBrainz + Cover Art Archive client; rate-limited to 1 req/1.1s per MusicBrainz policy; Wikipedia bio lookup via URL relations
- `internal/processor` — business logic: resolve artist by `MediaID` (or name fallback), enrich artist bio, match and enrich each album

### MusicBrainz integration

Artist search: `GET https://musicbrainz.org/ws/2/artist?query=artist:{name}&limit=1&fmt=json`

Artist URL relations (for Wikipedia): `GET https://musicbrainz.org/ws/2/artist/{mbid}?inc=url-rels&fmt=json`

Wikipedia extract: `GET https://en.wikipedia.org/w/api.php?action=query&prop=extracts&exintro=true&explaintext=true&titles={title}&format=json`

Album release groups: `GET https://musicbrainz.org/ws/2/release-group?artist={mbid}&type=album&inc=tags&limit=100&fmt=json`

Cover art: `GET https://coverartarchive.org/release-group/{release-group-mbid}` — returns JSON with `images[].image` and `images[].front`.

### Fields enriched

**Artist:**
| Field | Source |
|---|---|
| `bio` | Wikipedia intro section (via MusicBrainz URL relation) |
| `image_path` | preserved (no free image source available) |
| `name` | preserved |

**Album:**
| Field | Source |
|---|---|
| `year` | `first-release-date` from MusicBrainz release group |
| `genre` | highest-vote tag from MusicBrainz |
| `cover_path` | front image from Cover Art Archive |
| `title` | preserved |

Albums are matched to MusicBrainz release groups by normalized title (lowercase + trim). Unmatched albums are skipped silently.

### HTTP trigger

`POST /refresh/artist/{id}` re-fetches MusicBrainz metadata for the given artist ID and all their albums. Used by the admin refresh endpoint in river-api.
