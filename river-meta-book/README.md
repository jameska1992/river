# river-meta-book

Open Library metadata enrichment for audiobooks. Consumes `media.discovered.audiobook` events from RabbitMQ, resolves the audiobook record in `river-api`, queries Open Library, and updates the record with title / author / description / cover / year.

## Role in the platform

Twin of `river-meta-movie` / `river-meta-tv` for audiobooks. Runs alongside `river-audio-trans` on the same events. Uses Open Library — **no API key required**.

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
| `WORKER_COUNT` | `2` | |
| `PORT` | `8083` | Health check endpoint. |

## More

Search heuristics, cover-URL construction, and ACK contract in [`CLAUDE.md`](./CLAUDE.md).
