# river-meta-movie

TMDB metadata enrichment for movies. Consumes `media.discovered.movie` events from RabbitMQ, resolves the corresponding movie record in `river-api`, queries TMDB for title / description / year / genres / poster / backdrop / rating, and updates the record.

## Role in the platform

Runs in parallel with `river-video-trans` on the same events. Both consume every discovery event independently; if metadata lands before the transcoder has created the record, this service logs a warning and returns nil (ACK, no requeue). The scanner republishes on its next interval, so the retry is automatic.

## Commands

```bash
go build ./...
go vet ./...
go run ./cmd/server
go mod tidy
```

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `RIVER_API_USERNAME` | *(required)* | |
| `RIVER_API_PASSWORD` | *(required)* | |
| `RIVER_API_URL` | `http://localhost:8080` | |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | |
| `RABBITMQ_EXCHANGE` | `river.media` | |
| `TMDB_API_KEY` | *(required)* | v3 read-only key. |
| `TMDB_IMAGE_BASE` | `https://image.tmdb.org/t/p/original` | Stored on records at ingest time. |
| `WORKER_COUNT` | `2` | |

## More

Matching heuristics (title + year, title-only fallback), TMDB rate-limit behaviour, and the ACK/NACK contract in [`CLAUDE.md`](./CLAUDE.md).
