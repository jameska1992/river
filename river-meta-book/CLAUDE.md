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
PORT=8083
```

## Architecture

river-meta-book enriches audiobook records in river-api with metadata from Open Library (openlibrary.org). No API key required.

**Event flow:**
```
river-scan → RabbitMQ (exchange: river.media, key: media.discovered.audiobook)
                ├─→ river-audio-trans  (queue: river.audio.trans)
                └─→ river-meta-book    (queue: river.meta.book)
```

Both consumers receive every audiobook event independently. river-audio-trans creates the audiobook record; river-meta-book enriches it. If the record isn't found via `MediaID`, the processor logs a warning and ACKs (the scanner will re-publish on the next interval).

### Package responsibilities

- `internal/config` — env var loading; missing required vars fail at startup
- `internal/consumer` — AMQP wiring; queue `river.meta.book` binds `media.discovered.audiobook`; QoS prefetch=1; ACK on success, NACK (no requeue) on error
- `internal/apiclient` — river-api HTTP client; Bearer auth with automatic re-login on 401; `GetAudiobook`, `ListAudiobooks`, `UpdateAudiobook`
- `internal/openlib` — Open Library API client; searches by title, fetches description from works endpoint, constructs cover URL from cover ID
- `internal/processor` — business logic: resolves audiobook by `MediaID` (or title fallback), calls Open Library, writes enriched fields back via `UpdateAudiobook`

### Open Library integration

Search: `GET https://openlibrary.org/search.json?title={title}&fields=key,title,author_name,first_publish_year,subject,cover_i&limit=1`

Description: `GET https://openlibrary.org/works/{key}.json` — the `description` field is either a plain string or `{"type":"/type/text","value":"..."}`.

Cover image: `https://covers.openlibrary.org/b/id/{cover_i}-L.jpg`

### Fields enriched

| Field | Source |
|---|---|
| `author` | `author_name[0]` from search result |
| `description` | works endpoint |
| `year` | `first_publish_year` (falls back to existing if 0) |
| `genre` | `subject[0]` from search result |
| `cover_path` | cover URL constructed from `cover_i` |

Fields **not** overwritten: `title`, `narrator`, `duration` (duration is computed by the audio transcoder from the actual files).

### HTTP trigger

`POST /refresh/{id}` re-fetches Open Library metadata for the given audiobook ID. Used by the admin refresh button in river-api.
