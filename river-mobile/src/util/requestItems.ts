import type { MovieSearchResult, ShowSearchResult } from '../api'

export type RequestTab = 'movies' | 'shows'

// A search result normalised for rendering + requesting. `id` is the key the
// request API expects: tmdbId for movies, tvdbId for shows.
export interface RequestItem {
  id: number
  title: string
  year: number
  overview: string
  poster: string
  added: boolean
  kind: RequestTab
}

// Normalise the tab's results into a common list. Movies key on tmdbId, shows
// on tvdbId; the other tab's results are ignored.
export function requestItems(
  tab: RequestTab,
  movies: MovieSearchResult[],
  shows: ShowSearchResult[],
): RequestItem[] {
  return tab === 'movies'
    ? movies.map(m => ({ id: m.tmdbId, title: m.title, year: m.year, overview: m.overview, poster: m.poster, added: m.added, kind: 'movies' as const }))
    : shows.map(s => ({ id: s.tvdbId, title: s.title, year: s.year, overview: s.overview, poster: s.poster, added: s.added, kind: 'shows' as const }))
}
