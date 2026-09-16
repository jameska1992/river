// Small pure helpers shared across the browse UI. Kept dependency-free so
// they're easy to unit-test.

export type DetailMediaType = 'movie' | 'tvshow' | 'audiobook' | 'album' | 'artist'

// detailPath maps a media type + id to its detail route.
export function detailPath(mediaType: string, id: string): string {
  switch (mediaType) {
    case 'movie': return `/movies/${id}`
    case 'tvshow': return `/tvshows/${id}`
    case 'audiobook': return `/audiobooks/${id}`
    case 'album': return `/albums/${id}`
    case 'artist': return `/artists/${id}`
    default: return '/'
  }
}

// episodeCode formats a season/episode pair as SxxEyy.
export function episodeCode(season: number, episode: number): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `S${pad(season)}E${pad(episode)}`
}

// progressPct returns the 0–100 completion percentage, clamped and safe when
// duration is missing/zero.
export function progressPct(position: number, duration: number): number {
  if (!duration || duration <= 0) return 0
  return Math.min(100, Math.max(0, (position / duration) * 100))
}

// formatDuration renders seconds as H:MM:SS or M:SS.
export function formatDuration(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const pad = (n: number) => String(n).padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${m}:${pad(sec)}`
}

// initials returns 1–2 uppercase letters for an avatar placeholder.
export function initials(name: string): string {
  const parts = name.trim().split(/[\s._-]+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}
