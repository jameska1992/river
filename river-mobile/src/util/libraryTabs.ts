import type { LibraryType } from '../api'

export type LibraryTab = 'movies' | 'tvshows' | 'music' | 'audiobooks'

// Stable display order + labels, and how a library's type maps to a tab.
export const TAB_ORDER: LibraryTab[] = ['movies', 'tvshows', 'music', 'audiobooks']
export const TAB_LABEL: Record<LibraryTab, string> = { movies: 'Movies', tvshows: 'TV', music: 'Music', audiobooks: 'Books' }
const TAB_FOR_TYPE: Record<LibraryType, LibraryTab> = {
  movie: 'movies', tvshow: 'tvshows', music: 'music', audiobook: 'audiobooks',
}

// The tabs to show, in stable order, for the set of configured library types
// (deduped).
export function libraryTabs(types: LibraryType[]): LibraryTab[] {
  const present = new Set(types.map(t => TAB_FOR_TYPE[t]))
  return TAB_ORDER.filter(t => present.has(t))
}
