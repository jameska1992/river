# river-meta-tv

TMDB metadata enrichment for TV shows, seasons, and episodes. Consumes `media.discovered.tvshow` events from RabbitMQ, resolves the show + season in `river-api`, and updates records with TMDB metadata for each level.

## Role in the platform

The TV counterpart to `river-meta-movie`. Runs alongside `river-video-trans` on the same events; same "if the transcoder hasn't created the record yet, log-and-nil, scanner retries" semantics.

Handles the `SxxExx` filename convention for episode identification, and specials (season 0) matched by `source_path`.

## Commands

```bash
go build ./...
go vet ./...
go test ./...
go run ./cmd/server
```

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `RIVER_API_USERNAME` | *(required)* | |
| `RIVER_API_PASSWORD` | *(required)* | |
| `RIVER_API_URL` | `http://localhost:8080` | |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | |
| `RABBITMQ_EXCHANGE` | `river.media` | |
| `TMDB_API_KEY` | *(required)* | v3 read-only. |
| `TMDB_IMAGE_BASE` | `https://image.tmdb.org/t/p/original` | |
| `WORKER_COUNT` | `2` | |

## More

Show / season / episode resolution logic and the show-title-disambiguation-by-folder-path story live in [`CLAUDE.md`](./CLAUDE.md).
