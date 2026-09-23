# river-events

Joins media-lifecycle signals into a single **"ready to watch"** event, and is the
foundation for outbound integrations (webhooks, notifications, a Discord bot).

See the ADR: *river-events service owns the lifecycle join + outbound delivery*.

## What it does

The producer services already do their work independently and, on success, now emit
lifecycle events:

- `river-video-trans` / `river-audio-trans` → `media.transcoded.<type>`
- `river-meta-*` → `media.enriched.<type>`

`river-events` consumes both, and once a title has **both** a transcoded output and
enriched metadata, emits `media.ready.<type>`. It holds no state of its own — on each
event it reads the record back from `river-api` (the source of truth for both halves).

`river-api` deliberately has no RabbitMQ dependency, which is why the join lives here
rather than in the API.

## The event contract (`river.lifecycle` topic exchange)

Payloads are versioned (`schema_version`) — this is a semi-public surface, so consumers
must tolerate unknown fields and check `schema_version`.

Routing key: `<kind>.<type>` — e.g. `media.transcoded.movie`, `media.ready.tvshow`.

```jsonc
{
  "schema_version": 1,
  "kind": "media.ready",          // media.transcoded | media.enriched | media.ready
  "type": "tvshow",               // movie | tvshow | music | audiobook
  "library_id": "…",
  "media_id": "…",                // the unit: movie/episode/track/chapter, or the
                                  //   movie/show/album/audiobook that was enriched
  "parent_id": "…",               // enrichable parent of a transcoded unit:
                                  //   show (episode), album (track), audiobook (chapter)
  "season_id": "…",               // tvshow only
  "title": "…",                   // best-effort
  "occurred_at": "2026-09-23T…Z"
}
```

### `media_id` granularity

| type | transcoded | enriched | ready |
|---|---|---|---|
| movie | movie | movie | movie |
| tvshow | episode (`parent_id`=show) | show | episode (`parent_id`=show) |
| music | track (`parent_id`=album) | album | track (`parent_id`=album) |
| audiobook | chapter (`parent_id`=book) | audiobook | audiobook (book-level) |

### Readiness signals

A title is "ready" when transcoded **and** enriched. The enriched signal per type:

- **movie / tvshow** — `tmdb_id != 0`
- **music** — album `cover_path != ""` (Cover Art Archive; set only by enrichment)
- **audiobook** — `open_library_key != ""`

Because transcode is per-unit and enrichment is per-parent, an `enriched` event fans
out over the parent's already-transcoded units (so nothing is missed when enrichment
lands after the files). Emission is de-duplicated in-process; **consumers must be
idempotent** (a restart clears the guard).

## Configuration

| env | default | notes |
|---|---|---|
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | |
| `RIVER_API_URL` | `http://localhost:8080` | |
| `RIVER_API_USERNAME` / `RIVER_API_PASSWORD` | — | required unless `RIVER_API_KEY` is set |
| `RIVER_API_KEY` | — | optional per-service key (service-role Phase 2) |
| `WORKER_COUNT` | `2` | |
| `MAX_RETRIES` | `5` | before a message is dead-lettered |
| `RETRY_BACKOFF_SECONDS` | `30` | |
