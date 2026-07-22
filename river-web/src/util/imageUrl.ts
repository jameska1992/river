// Rewrite the TMDB `/t/p/<size>/` segment on image URLs coming back
// from the api so we request a size actually useful for the display,
// not the multi-megabyte `/t/p/original/` raw upload the metadata
// services store at ingest.
//
// The metadata services (river-meta-movie, river-meta-tv) build fully
// qualified URLs at scrape time using TMDB_IMAGE_BASE, currently
// hard-coded to `.../t/p/original`. That's fine for archival storage
// but wasteful to render — a card poster is ~200 CSS px wide, so
// w342 is the right size. Fixing it at the client is a one-file
// change; fixing it at ingest would need a re-scrape of every record.
//
// Non-TMDB URLs (OpenLibrary book covers, local music art, data:/
// blob:, missing values) pass through untouched.

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
  if (/^(data:|blob:)/i.test(trimmed)) return trimmed

  let host: string
  try {
    host = new URL(trimmed).hostname
  } catch {
    return trimmed
  }
  if (host !== 'image.tmdb.org') return trimmed

  const target = variant === 'backdrop' ? TMDB_BACKDROP_SIZE : TMDB_POSTER_SIZE
  return trimmed.replace(/\/t\/p\/[^/]+\//, `/t/p/${target}/`)
}
