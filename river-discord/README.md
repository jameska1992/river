# river-discord

Posts Discord notifications when River media becomes **ready to watch** — a rich
embed (poster, year, synopsis, genres) built from the river-api record.

It receives River's outbound webhooks (see #195) over HTTP, verifies the HMAC
signature, reads the enriched record from river-api, and posts an embed to a
Discord [incoming webhook](https://support.discord.com/hc/en-us/articles/228383668).
No Discord bot token or gateway connection is required.

## Setup

1. **Create a Discord incoming webhook** for the channel you want posts in
   (Discord: *Channel → Edit → Integrations → Webhooks → New Webhook → Copy URL*).
   This is `DISCORD_WEBHOOK_URL`.

2. **Mint a read-only River API token** in the admin UI (*Admin → API Tokens →
   Mint*). Copy the `rvat_…` value into `RIVER_API_TOKEN`. It's used only to read
   media records for the embed.

3. **Create a River webhook** pointing at this service (*Admin → Webhooks → Add*):
   - **URL:** `http://river-discord:8090/hooks/river` (the in-network address; the
     compose service name is `river-discord`).
   - **Events:** leave empty (all) or select `media.ready`.
   - Copy the shown `whsec_…` secret into `RIVER_WEBHOOK_SECRET` — it's shown once.

4. **Set the env** (e.g. in your root `.env`) and start the service under the
   `discord` compose profile:

   ```bash
   RIVER_DISCORD_API_TOKEN=rvat_…
   RIVER_DISCORD_WEBHOOK_SECRET=whsec_…
   DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/…

   docker compose --profile discord up -d river-discord
   ```

## Routing to multiple channels

Set `DISCORD_ROUTES` to a JSON object mapping a **library id** or a **media type**
(`movie` | `tvshow` | `music` | `audiobook`) to a Discord webhook URL. It's
checked library-id first, then type, then falls back to `DISCORD_WEBHOOK_URL`:

```json
{ "movie": "https://discord.com/api/webhooks/AAA",
  "music": "https://discord.com/api/webhooks/BBB",
  "0f3c…lib-id": "https://discord.com/api/webhooks/CCC" }
```

## Behaviour notes

- A season's episodes (and an album's tracks) that go ready together are
  **coalesced into one message** — see `BATCH_WINDOW_SECONDS` (default 10s).
- By default it notifies only on `media.ready` (`NOTIFY_KINDS`).
- Poster/backdrop images come straight from the record's public TMDB URLs; a
  local/un-enriched cover is omitted rather than shown broken.

See `CLAUDE.md` for architecture and the full environment-variable table.
