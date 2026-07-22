# river-meta-music

MusicBrainz metadata enrichment for music. Consumes `media.discovered.music` events from RabbitMQ, resolves Artist / Album records in `river-api`, queries MusicBrainz for artist and album details, pulls covers from the Cover Art Archive, and pulls artist bios from Wikipedia via MusicBrainz URL relations.

## Role in the platform

Twin of `river-meta-movie` / `river-meta-tv` / `river-meta-book` for music. Runs alongside `river-audio-trans` on the same events. Uses MusicBrainz — **no API key required**.

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
| `PORT` | `8084` | Health check endpoint. |

## More

Artist / album disambiguation, cover-art fallback order, Wikipedia bio scraping, and the ACK contract in [`CLAUDE.md`](./CLAUDE.md).

Note MusicBrainz has a strict 1 rps rate limit — the client honours it internally.
