# river-scan

Filesystem scanner for River. Walks configured library paths, hashes files for change detection, and publishes `media.discovered.*` events to RabbitMQ so downstream transcoders and metadata services pick them up.

## Role in the platform

Sits between the filesystem and the ingest fan-out. Reads library configuration from `river-api` (login-authenticated), doesn't touch the DB directly. Two "flavours" of downstream:

- **Transcode mode** (default) — publishes events; consumers create records.
- **Direct mode** (`DISABLE_TRANSCODING=1` or per-library `pre_transcoded=true`) — scanner writes records to `river-api` directly. Pre-transcoded still publishes for metadata; the transcoders no-op on the event.

Also registers sidecar files that live alongside a movie/episode:
- `.audio_N.mp4` variants (transcoder output convention) → `AudioTrack` records.
- `subtitles/*.vtt` files in a case-insensitive subdir → `Subtitle` records.

Both are idempotent per file path.

## Commands

```bash
go build ./...
go vet ./...
go test ./...
go test ./internal/scanner/... -v      # scanner covers walker + sidecar tests
```

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `RIVER_API_USERNAME` | *(required)* | |
| `RIVER_API_PASSWORD` | *(required)* | |
| `RIVER_API_URL` | `http://localhost:8080` | |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | |
| `RABBITMQ_EXCHANGE` | `river.media` | Topic. |
| `SCAN_INTERVAL` | *(unset — one-shot)* | Seconds between full scan cycles. |
| `STATE_PATH` | `scanner-state.json` | Change-detection state file. |
| `DISABLE_TRANSCODING` | *(unset)* | Any non-empty value flips to direct mode globally. |
| `MAX_SCAN_DEPTH` | `6` | Recursive walk cap. |

## More

Scan flow, per-library-type handling, and the "movie files are keyed per-file / TV/audiobook/music dirs are keyed per-dir" convention live in [`CLAUDE.md`](./CLAUDE.md).
