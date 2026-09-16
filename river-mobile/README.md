# river-mobile

Touch-first web client for River, built for phones/tablets. React + Vite.
Used standalone in a mobile browser, or bundled inside `river-mobile-android`
(planned) as the WebView payload — the same pattern as
[`river-tv`](../river-tv/) → [`river-tv-android`](../river-tv-android/).

## Status

Scaffold (issue #141 of the Mobile app epic, #148). Working: build setup,
API client, single-account login with a configurable server URL, and the
bottom-tab app shell (Home / Search / Library / Watchlist / Settings). The
browse pages, players, and background audio are tracked as follow-ups
(#142–#147).

## Relationship to the other clients

- **API client + types** (`src/api/`) are reused from `river-tv` — the
  WebView-appropriate client with server-URL configuration and full endpoint
  coverage.
- **Design tokens** (`src/index.css`) mirror river-web / river-tv but are
  sized phone-first (16px base, natural scrolling) rather than 10-foot TV.
- Navigation is a bottom tab bar (touch), not river-tv's D-pad focus manager.

## Commands

```bash
npm install
npm run dev      # Vite dev server on :5175 (proxies /api → RIVER_API_TARGET or localhost:8080)
npm run build    # tsc -b && vite build → dist/
npm run lint
```

Point at a specific backend in dev:

```bash
RIVER_API_TARGET=http://192.168.1.10:8080 npm run dev
```

At runtime the server URL is set on the login screen and persisted in
`localStorage` (same as river-tv).
