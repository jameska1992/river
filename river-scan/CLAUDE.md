# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...        # build
go vet ./...          # lint
go test ./...         # test
go test ./internal/scanner/... # test a single package
```

## What This Service Does

`river-scan` is a media filesystem scanner for the River streaming platform. It fetches library configuration from `river-api`, walks the library paths on disk, and publishes `media_discovered` events to a RabbitMQ topic exchange for downstream consumers (transcoders, metadata enrichment). It does **not** write to river-api itself.

## Architecture

```
cmd/server/main.go       entry point — wires deps, runs scan loop, handles signals
internal/config          env-var config
internal/apiclient       HTTP client for river-api (login + GET /api/libraries)
internal/scanner         core scan logic
internal/publisher       RabbitMQ publisher
internal/state           JSON file tracking known directories by content hash
```

**Scan flow:** `Scanner.Run` logs in to river-api, fetches all libraries, builds a per-run `scanCache` (movie list cached once per library so flat scans don't paginate /api/movies N times), then calls `scanLibrary` → `scanPath` for each configured path. State is flushed to disk once at the end of `Run` rather than on every record — see `state.Flush`.

**Movie libraries are file-keyed.** `scanPath` for `movie` libraries runs `walkMovieFiles` (`walker.go`): a depth-capped recursive walk that emits one event per video file. This tolerates flat layouts (`Movies/Foo.mkv`), one-folder-per-movie (`Movies/Foo (2020)/file.mkv`), nested categories (`Movies/Action/Foo (2020)/file.mkv`), and any mix. Title/year/IDs are parsed from the file's own basename first, then layered with the parent directory's name and any NFO sidecar (via `internal/nameinfo`). State is keyed per-file path. Each file becomes one `MediaDiscoveredEvent` with `Files: []string{path}`.

The walker skips:
- system/hidden dirs (denylist in `walker.go`: `@eaDir`, `.AppleDouble`, `lost+found`, anything starting with `.`)
- Plex-convention extras subdirs (`Extras/`, `Featurettes/`, `Trailers/`, `Behind The Scenes/`, etc.)
- Plex-convention extras files by filename suffix (`*-trailer.mkv`, `*-behindthescenes.mkv`, `Sample.mkv`, etc.)

**Non-movie libraries are still dir-keyed.** TV shows (`scanTVShow` → `scanSeason` per `Show/Season N/`), audiobooks (`scanAudiobook` per top-level dir), and music (`scanMusic` per artist dir) keep their dir-based grouping because their downstream consumers depend on the dir-level grouping (seasons → episodes; artist → albums → tracks).

**Change detection** uses a SHA-256 hash of file paths + sizes + mod times. For movies the hash covers a single file; for the others it covers all media files under the dir. A changed hash re-publishes.

**TV show structure:** `scanTVShow` reads `Show/Season N/` and emits events with both `directory_name`/`directory_path` (the show) and `season_name`/`season_path` (the season). State is keyed on the season path.

**RabbitMQ:** topic exchange, routing key `media.discovered.<library_type>`. Messages are persistent JSON `MediaDiscoveredEvent`.

## Sibling Service

`river-api` lives at `../river-api`. Libraries have types `movie | tvshow | music | audiobook` and store `paths` as a JSON-encoded string column (arrives from the API as a JSON string, not an array — `Library.ParsedPaths()` handles this). Login uses `username` + `password` (not email).

## Environment Variables

| Variable | Default | Required |
|---|---|---|
| `RIVER_API_USERNAME` | — | yes |
| `RIVER_API_PASSWORD` | — | yes |
| `RIVER_API_URL` | `http://localhost:8080` | no |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | no |
| `RABBITMQ_EXCHANGE` | `river.media` | no |
| `SCAN_INTERVAL` | *(none — run once and exit)* | no |
| `STATE_PATH` | `scanner-state.json` | no |
| `DISABLE_TRANSCODING` | *(unset)* | no |
| `MAX_SCAN_DEPTH` | `6` | no |

When `DISABLE_TRANSCODING` is set (any non-empty value), the scanner skips RabbitMQ entirely and registers media records directly in river-api with original file paths. Movies update one record per video file; TV episodes are parsed from filenames; music files are grouped by subdirectory into albums; audiobook files become chapters sorted by filename.

`MAX_SCAN_DEPTH` caps how deep the recursive movie walker descends below the library root. Defaults to 6, which comfortably covers `Movies/Genre/Sub/Title/file.mkv` layouts.
