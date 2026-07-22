# river-tv

TV-optimised web client for River. React + Vite. Designed for D-pad remote navigation on a 1080p 10-foot UI. Used standalone in a TV browser, or bundled inside [`river-tv-android`](../river-tv-android/) as the WebView payload.

## Development

```bash
npm install
npm run dev      # http://localhost:5174, proxies /api → localhost:8080 (override with RIVER_API_TARGET)
npm run build    # production bundle in dist/
npm run lint
```

## Configuration

The API base URL is stored in `localStorage` under `river:api-base`. The default is `http://localhost:8080/api` — set from the login-screen Server picker at runtime. Change the built-in default in `src/api/client.ts` if you deploy against a different address.

## Key pieces

- **`src/hooks/useFocus.tsx`** — the spatial focus manager. Every focusable element registers itself; arrow keys pick the nearest registered element by geometric distance. Enter fires `onSelect`; Back / Escape fires `onBack` (falling back to focusing an entry tagged with `backFocusesTag`). See the comments in that file — the abstraction is what makes remote-friendly TV UIs practical.
- **`src/util/imageUrl.ts`** — routes TMDB URLs through the `/api/image` proxy AND rewrites `/t/p/original/` to `w342` (posters) / `w1280` (backdrops). Fire TV WebViews are unreliable at reaching the TMDB CDN directly, so proxying is not optional here.
- **`src/pages/`** — one page per route. `BrowsePage` is a generic paginated grid used by Movies / TV Shows / Audiobooks; the more specialised `CollectionsPage` / `WatchlistPage` / `SearchPage` are separate.
- **`src/components/PlayerScreen.tsx`** — the shared video/audio player. Resumes from server-stored progress, extracts subtitle tracks, wires D-pad seek.
- **`src/components/Popup.tsx`** — modal wrapper that pauses the outer `FocusProvider` and runs its own scope. Fixes the "arrow keys leak past the popup" class of bugs.

## Remote navigation contract

- Arrow keys move focus by geometric proximity.
- OK fires the focused item's action.
- Back on the browse / search / watchlist / collections pages navigates to the home page. On detail pages it goes back one history entry. In text inputs, Back blurs the input first (a second Back closes the surrounding popup / navigates home).
- The double-Back exit gesture only fires at the root (`/`) or login (`/login`) URL — otherwise Back is captured by the JS-layer navigation handlers.

## Deploying

- **As a plain web app**: `npm run build`, host `dist/` behind any static server. Any TV with a modern browser (Chrome / Firefox / Silk) will load it.
- **As a Fire TV / Android TV native app**: use [`river-tv-android`](../river-tv-android/), which bundles the built `dist/` inside an APK and serves it through `WebViewAssetLoader`. Zero external hosting required.
