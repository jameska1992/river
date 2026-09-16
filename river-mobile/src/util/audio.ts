import { api } from '../api'
import type { Album, Track, Audiobook, AudiobookChapter } from '../api'

// One playable audio item in the player's queue. Artwork is kept as a raw path
// and run through imageUrl() at render / metadata time.
export interface AudioItem {
  id: string
  streamUrl: string
  title: string
  artist: string
  album: string
  artworkPath: string
  // Only audiobook chapters have server-side progress (WatchProgress
  // media_type 'chapter'); music tracks have none, so this is left unset.
  progressKind?: 'chapter'
}

export function trackToAudioItem(track: Track, album: Album, artistName: string): AudioItem {
  return {
    id: track.id,
    streamUrl: api.trackStreamUrl(track.id),
    title: track.title,
    artist: artistName,
    album: album.title,
    artworkPath: album.cover_path,
  }
}

export function chapterToAudioItem(chapter: AudiobookChapter, book: Audiobook): AudioItem {
  return {
    id: chapter.id,
    streamUrl: api.chapterStreamUrl(book.id, chapter.id),
    title: chapter.title || `Chapter ${chapter.number}`,
    artist: book.author,
    album: book.title,
    artworkPath: book.cover_path,
    progressKind: 'chapter',
  }
}

// Next/previous index within [0, length); null at the ends (so callers can
// stop rather than wrap).
export function stepIndex(index: number, delta: number, length: number): number | null {
  const next = index + delta
  return next >= 0 && next < length ? next : null
}
