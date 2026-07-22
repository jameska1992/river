# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...          # build
go vet ./...            # static analysis
go test ./...           # test
go run ./cmd/server     # run locally
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
OUTPUT_DIR=          # empty = transcode output placed next to source file
```

## Architecture

river-audio-trans consumes `media.discovered.music` and `media.discovered.audiobook` events from the `river.audio.trans` queue, transcodes audio files to AAC .m4a where needed, and registers records in river-api.

```
RabbitMQ (media.discovered.music / media.discovered.audiobook)
  └── consumer.Consumer
        └── processor.Handle()
              ├── transcoder.Probe() / Transcode()   # ffprobe + ffmpeg
              └── apiclient.Client                   # CRUD against river-api
```

### Package responsibilities

- **`internal/config`** — env var loading; missing required vars fail at startup
- **`internal/consumer`** — AMQP wiring; queue `river.audio.trans` binds both routing keys; QoS prefetch=1; ACK on success, NACK (no requeue) on error
- **`internal/transcoder`** — `Probe` returns codec + duration via ffprobe; `NeedsTranscode` returns true if not already AAC in an .m4a container; `Transcode` runs `ffmpeg -vn -c:a aac -b:a 256k`
- **`internal/apiclient`** — thread-safe HTTP client with Bearer auth and automatic re-login on 401; paginated list endpoints at 200 items/page
- **`internal/processor`** — all business logic; other packages are pure I/O

### Music processing

One event = one artist directory. `DirectoryName` is treated as the artist name. Files are grouped by their immediate subdirectory under `DirectoryPath` — each subdirectory becomes an album (title and optional year parsed from `"Title (YYYY)"`). Files directly in `DirectoryPath` are grouped under an album named after the artist. Track numbers are parsed from filename prefixes (`01 - Title`, `01. Title`, etc.); deduplication is by track number within the album.

### Audiobook processing

One event = one audiobook directory. `DirectoryName` becomes the audiobook title (year stripped from `"Title (YYYY)"`). All files are chapters, sorted by filename. Chapter numbers are parsed from filename prefixes (`01`, `Chapter 01`, `Part 01`); if unparseable, the 1-based sort position is used. Deduplication is by chapter number.

### Concurrency model

`WORKER_COUNT` goroutines each hold an independent AMQP connection+channel. RabbitMQ round-robins deliveries. A worker crash propagates a fatal error to main, which exits the process.

### System dependencies

`ffmpeg` and `ffprobe` must be installed and on `PATH`.
