# river-audio-trans

Audio transcoder for River. Consumes `media.discovered.music` and `media.discovered.audiobook` events from RabbitMQ, transcodes to AAC in an .m4a container where needed, and registers Artist / Album / Track / Audiobook / Chapter records with `river-api`.

## Role in the platform

Twin of `river-video-trans`, but audio-only. Same pre-transcoded-library short-circuit — events with `pre_transcoded=true` are ACKed and no-op.

## Commands

```bash
go build ./...
go vet ./...
go test ./...
go run ./cmd/server
```

`ffmpeg` and `ffprobe` must be on PATH.

## Ingest conventions

- **Music**: one event = one artist directory. Files are grouped by immediate subdirectory into albums; loose files at the artist root form an album named after the artist. Track numbers parsed from filename prefix (`01 - Title`, `01. Title`, …). Dedup by track number within album.
- **Audiobooks**: one event = one audiobook directory. Files are chapters, sorted by filename. Chapter numbers parsed from filename prefix (`01`, `Chapter 01`, `Part 01`); unparseable → 1-based sort index.

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `RIVER_API_USERNAME` | *(required)* | |
| `RIVER_API_PASSWORD` | *(required)* | |
| `RIVER_API_URL` | `http://localhost:8080` | |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | |
| `RABBITMQ_EXCHANGE` | `river.media` | |
| `WORKER_COUNT` | `2` | |
| `OUTPUT_DIR` | *(unset — output beside source)* | |

## More

Concurrency model + per-type parsing details in [`CLAUDE.md`](./CLAUDE.md).
