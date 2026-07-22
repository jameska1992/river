# river-web

Browser client for River. React + Vite. Also the admin surface (libraries, users, upload, logs, scanner state, unidentified items).

## Development

```bash
npm install
npm run dev      # http://localhost:5173, proxies /api → localhost:8080 (see vite.config.ts)
npm run build    # production bundle in dist/
npm run lint
npm run preview  # preview the production bundle
```

The dev server proxies `/api` to `http://localhost:8080` by default (see `vite.config.ts`). Point it at your own river-api instance with `RIVER_API_TARGET=http://<host>:8080 npm run dev`.

## Configuration

The API base URL used at runtime is stored in `localStorage` under `river:api-base`. In dev the default is `/api` (which the Vite proxy handles). For a production deployment, the same origin is expected to serve both the built assets and the `/api` prefix (e.g. via nginx routing).

## Deploying

```bash
npm run build
# copy dist/ to your web root
```

The provided `Dockerfile` builds an nginx image serving `dist/` — used by the top-level `docker-compose.yml`.

## Key pieces

- **`src/api/`** — typed API client, auth, WebSocket for progress updates. `client.ts` is where every request is issued.
- **`src/context/`** — React Context providers wrapping the API (`AuthContext`, `LibrariesContext`, `MoviesContext`, `TVShowsContext`, `MusicContext`, `AudiobooksContext`, `WatchlistContext`).
- **`src/pages/`** — one folder per page. `admin/` contains the admin-only pages behind the `RequireAdmin` guard.
- **`src/components/`** — shared UI (`MediaCard`, `HeroBanner`, `CollectionPreview`, `ContinueWatching`, `CastButton`).
- **`src/util/imageUrl.ts`** — rewrites TMDB `/t/p/original/` URLs to per-usage sizes (`w342` posters, `w1280` backdrops); passes non-TMDB hosts through untouched.
- **`src/hooks/useCast.ts` + `src/components/CastButton.tsx`** — Google Cast integration. Chrome-only. See [Chromecast section](#chromecast) below.

## Chromecast

Cast buttons appear on the movie, episode, music, and audiobook player pages **when the browser has the Google Cast Web SDK available** (Chrome / Edge). The receiver is the default media receiver — anything the transcoder outputs (H.264/AAC MP4, AAC .m4a) plays without a custom receiver app.

Fire TV / Roku don't speak Google Cast — for a TV client, use [`river-tv-android`](../river-tv-android/) instead.

## Where the admin bits live

- `pages/admin/LibrariesPage.tsx` — create / edit / delete libraries. The "Already transcoded" checkbox marks a library as pre-transcoded (see repo-level [README](../README.md) architecture section).
- `pages/admin/UsersPage.tsx` — user management.
- `pages/admin/UploadPage.tsx` — direct upload of media files.
- `pages/admin/ScannerStatePage.tsx` — inspect / edit `river-scan`'s content-hash state.
- `pages/admin/UnidentifiedPage.tsx` — TMDB match failures, with a picker to manually identify.
- `pages/admin/LogsPage.tsx` — cross-service log stream.
