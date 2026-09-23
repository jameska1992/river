# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...          # build
go test ./...           # run all tests
go test -race ./...     # the batcher/notifier are concurrent — run with -race
go vet ./...            # static analysis
go run ./cmd/server     # run the service
```

This module has **no external dependencies** — stdlib only (so `go.sum` is empty).
Keep it that way unless there's a strong reason; a Discord embed is just JSON.

## What this service does

`river-discord` posts Discord notifications when River media becomes **ready to
watch**. It is an HTTP receiver for River's outbound webhooks (#195), **not** a
RabbitMQ consumer and **not** a Discord gateway bot:

```
river-events ──(signed webhook POST: LifecycleEvent JSON)──▶ river-discord ──reads record──▶ river-api
   media.ready.*                                                   │
                                                                   └── embed ──▶ Discord incoming-webhook URL(s)
```

The POST body is exactly the `LifecycleEvent` envelope river-events sends; it is
signed `X-River-Signature: sha256=<hmac>` with the webhook's `whsec_…` secret.

## Architecture

One-directional flow, each package depends only on those below it:

```
server  →  notifier  →  { apiclient, embed, discord, batcher }
                              (events is the shared payload contract)
```

- **`internal/events`** — mirrors river-events' `LifecycleEvent` contract (this
  is a separate module, so it's duplicated, not imported). **Keep in lockstep
  with `river-events/internal/events`.**
- **`internal/server`** — HTTP receiver: verifies the HMAC signature, parses the
  event, filters unknown kinds, hands off to a `Handler`, acks `202`. Holds no
  business logic. `/healthz` for the container healthcheck.
- **`internal/apiclient`** — reads records from river-api with a Bearer `rvat_…`
  token; only the fields embeds render.
- **`internal/embed`** — **pure** Discord message/embed builders per media type.
  No HTTP, no config — trivially testable. `imageURL` only emits absolute
  http(s) URLs so a local/un-enriched cover doesn't render broken.
- **`internal/discord`** — POSTs to a Discord webhook URL; honours `429`
  (Retry-After), retries 5xx/transport, treats other 4xx as permanent.
- **`internal/batcher`** — debounce coalescer: events sharing a key flush after a
  quiet window, with a max-size early flush so a large import still delivers.
- **`internal/notifier`** — the glue: filter → batch (by announce entity) →
  enrich → embed → route → deliver.

## Key design points

- **Batch by announce entity, not per child.** `media.ready.tvshow` fires
  per-episode and `media.ready.music` per-track; the notifier keys the batcher on
  the show/album so a season lands as **one** digest message and an album as one.
  `announceKey` in `notifier.go` owns this.
- **Routing precedence:** library id → media type → default channel
  (`DISCORD_ROUTES` maps either key to a webhook URL).
- **Read failures drop, they don't retry.** river-events already retries webhook
  delivery with its own DLQ; re-erroring here would double up. A failed record
  read is logged and skipped.
- **Notified kinds are configurable** (`NOTIFY_KINDS`, default `media.ready`).
  "Added" on `media.discovered.*` is intentionally **not** covered — those aren't
  webhook events; see the #196 plan for the follow-up option.

## Environment Variables

| Var | Required | Default | Purpose |
|---|---|---|---|
| `PORT` | no | `8090` | HTTP listen port |
| `WEBHOOK_PATH` | no | `/hooks/river` | Route river-events POSTs to |
| `RIVER_WEBHOOK_SECRET` | **yes** | — | `whsec_…` secret to verify `X-River-Signature` |
| `RIVER_API_URL` | no | `http://localhost:8080` | river-api base URL |
| `RIVER_API_TOKEN` | **yes** | — | read-only `rvat_…` token for record lookups |
| `DISCORD_WEBHOOK_URL` | **yes** | — | default Discord channel webhook |
| `DISCORD_ROUTES` | no | — | JSON `{ "<lib-id-or-type>": "<url>" }` |
| `BATCH_WINDOW_SECONDS` | no | `10` | quiet-period before flushing a burst |
| `NOTIFY_KINDS` | no | `media.ready` | comma-separated lifecycle kinds to notify on |

See `README.md` for the one-time setup (create webhook → copy secret → mint token).
