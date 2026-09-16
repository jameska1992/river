// Normalize an image path coming back from the river-api and route
// external CDN URLs through river-api's /image proxy.
//
// Why the proxy: WebViews on Android TV / Fire TV often can't reach
// the TMDB image CDN reliably (TLS handshakes time out, requests get
// cancelled), even when the host machine's browser can. Funnelling
// every poster through the API (which itself sits on the LAN and has
// a fast HTTPS path to TMDB) sidesteps the WebView's CDN-reachability
// quirks and lets us cache the bytes for free at the same time.
//
// Records that arrive as a bare TMDB path ("/abc.jpg") — usually
// older enrichments — get prefixed with the TMDB image base before
// being proxied. data:/blob: URLs and full URLs from non-CDN hosts
// pass through untouched.
//
// TMDB size handling: the metadata services store fully-qualified
// TMDB URLs at ingest time, currently all with the '/t/p/original/'
// segment (~2000-4000 px wide raw uploads). That's an order of
// magnitude bigger than we ever need to display and hammers decode
// time on Fire TV. We rewrite the size segment here based on the
// caller's variant — no DB migration, no re-scrape. Available TMDB
// sizes: w92, w154, w185, w342, w500, w780, w1280 (backdrops only),
// original.
import { api } from '../api'

const TMDB_IMAGE_BASE = 'https://image.tmdb.org/t/p/original'
// Upstream hosts whose images we route through /api/image. Must match
// the allow-list in river-api/internal/handlers/image_proxy.go.
const PROXIED_HOSTS = new Set([
  'image.tmdb.org',          // movies + TV (TMDB)
  'covers.openlibrary.org',  // audiobook covers (Open Library)
])

// TMDB size to request per variant. Posters render at card widths
// (~250 CSS px) so w342 is right-sized — anything larger just makes
// the WebView decode more pixels than the panel can display.
// Backdrops are used as full-width heroes (1920 CSS px on our design
// canvas) so w1280 keeps enough resolution to look sharp when the
// browser upscales it slightly, without ballooning bytes.
const TMDB_POSTER_SIZE = 'w342'
const TMDB_BACKDROP_SIZE = 'w1280'

export type ImageVariant = 'poster' | 'backdrop'

export function imageUrl(
  src: string | null | undefined,
  variant: ImageVariant = 'poster',
): string | undefined {
  if (!src) return undefined
  const trimmed = src.trim()
  if (trimmed === '') return undefined

  // data: / blob: are already self-contained, never proxy them.
  if (/^(data:|blob:)/i.test(trimmed)) return trimmed

  // A leading slash is the legacy TMDB-bare-path case. Build the full
  // TMDB URL, then fall through to the proxy branch below.
  const absolute = /^https?:\/\//i.test(trimmed)
    ? trimmed
    : trimmed.startsWith('/') ? TMDB_IMAGE_BASE + trimmed : trimmed

  // Anything not from a known CDN passes through.
  let host: string
  try {
    host = new URL(absolute).hostname
  } catch {
    return absolute
  }
  if (!PROXIED_HOSTS.has(host)) return absolute

  const sized = host === 'image.tmdb.org'
    ? resizeTmdb(absolute, variant)
    : absolute

  return `${api.apiBaseURL}/image?url=${encodeURIComponent(sized)}`
}

// Rewrite the TMDB size segment. TMDB URLs look like
// https://image.tmdb.org/t/p/<size>/<hash>.jpg — we just swap the
// <size> token. Robust to whatever base the metadata services
// happened to write (original / w780 / etc.); if the URL doesn't
// match the expected shape we return it unchanged.
function resizeTmdb(url: string, variant: ImageVariant): string {
  const target = variant === 'backdrop' ? TMDB_BACKDROP_SIZE : TMDB_POSTER_SIZE
  return url.replace(/\/t\/p\/[^/]+\//, `/t/p/${target}/`)
}
