# river-video-trans

Video transcoder for River. Consumes `media.discovered.movie` and `media.discovered.tvshow` events from RabbitMQ, transcodes source files to H.264/AAC/MP4 where needed, and registers the resulting media records with `river-api`.

## Role in the platform

One of the downstream consumers on the ingest exchange (see repo-level [README](../README.md)). Also handles subtitle extraction (embedded streams + sidecar files) and per-audio-track variant MP4 creation.

**Pre-transcoded libraries**: events with `pre_transcoded=true` are ACKed and no-op — the scanner has already registered the media record and any sidecars.

## Commands

```bash
go build ./cmd/server
go vet ./...
go test ./...
go mod tidy
```

`ffmpeg` and `ffprobe` must be on PATH.

## Encode paths (in preference order)

1. **NVENC + CUDA decode** — full GPU pipeline. Requires NVDEC support for the input codec.
2. **NVENC + CPU decode** — fallback when NVDEC can't handle the input.
3. **libx264** — CPU only.

Selection is automatic based on what `ffprobe` reports and what NVENC accepts.

Source files that are already H.264/AAC/MP4 at ≤1080p are **copied**, not re-encoded. If the source path happens to equal the output path (e.g. `OUTPUT_DIR` unset → output beside source), the copy is skipped entirely.

## Output layout

Movies land at `{OUTPUT_DIR}/movies/{Title} ({Year})/{Title}.mp4`.
Episodes land at `{OUTPUT_DIR}/shows/{ShowName}/Season {N}/S{ss:02}E{ee:02}.mp4`.

`OUTPUT_DIR` unset → beside the source. Title / show name are pulled from the resolved `river-api` record so admin renames propagate to disk on the next scan.

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `RIVER_API_USERNAME` | *(required)* | |
| `RIVER_API_PASSWORD` | *(required)* | |
| `RIVER_API_URL` | `http://localhost:8080` | |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | |
| `RABBITMQ_EXCHANGE` | `river.media` | |
| `WORKER_COUNT` | `2` | Independent goroutines each with their own AMQP conn. |
| `OUTPUT_DIR` | *(unset — output beside source)* | Canonical output root. |

## More

`CLAUDE.md` covers the concurrency model, the exact `NeedsTranscode` heuristic, sidecar detection, and the audio-variant / subtitle registration flow.
