import { describe, it, expect } from 'vitest'
import { trackToAudioItem, chapterToAudioItem, stepIndex, type AudioItem } from './audio'
import type { Album, Track, Audiobook, AudiobookChapter } from '../api'

const base = { created_at: '', updated_at: '' }

const album = { ...base, id: 'al1', library_id: 'l', artist_id: 'ar1', title: 'Kind of Blue', year: 1959, genre: 'Jazz', cover_path: '/cover.jpg' } as Album
const track = { ...base, id: 'tr1', library_id: 'l', album_id: 'al1', artist_id: 'ar1', title: 'So What', number: 1, disc_number: 1, duration: 545, file_path: '/x.m4a' } as Track

const book = { ...base, id: 'bk1', library_id: 'l', title: 'Dune', author: 'Frank Herbert', narrator: 'N', description: '', year: 1965, genre: 'SF', cover_path: '/dune.jpg', duration: 72000 } as Audiobook
const chapter = { ...base, id: 'ch1', audiobook_id: 'bk1', number: 3, title: 'Arrakis', duration: 1800, file_path: '/c.m4a' } as AudiobookChapter

describe('trackToAudioItem', () => {
  it('maps a track + album + artist name', () => {
    const item = trackToAudioItem(track, album, 'Miles Davis')
    expect(item.id).toBe('tr1')
    expect(item.title).toBe('So What')
    expect(item.artist).toBe('Miles Davis')
    expect(item.album).toBe('Kind of Blue')
    expect(item.artworkPath).toBe('/cover.jpg')
    expect(item.progressKind).toBeUndefined() // tracks have no server progress
    expect(item.streamUrl).toContain('tr1')
  })
})

describe('chapterToAudioItem', () => {
  it('maps a chapter + book, tagged for chapter progress', () => {
    const item = chapterToAudioItem(chapter, book)
    expect(item.id).toBe('ch1')
    expect(item.title).toBe('Arrakis')
    expect(item.artist).toBe('Frank Herbert')
    expect(item.album).toBe('Dune')
    expect(item.progressKind).toBe('chapter')
    expect(item.streamUrl).toContain('ch1')
  })
  it('falls back to a numbered title when untitled', () => {
    const item = chapterToAudioItem({ ...chapter, title: '' }, book)
    expect(item.title).toBe('Chapter 3')
  })
})

describe('stepIndex', () => {
  it('advances and rewinds within bounds', () => {
    expect(stepIndex(0, 1, 3)).toBe(1)
    expect(stepIndex(2, -1, 3)).toBe(1)
  })
  it('returns null at the ends', () => {
    expect(stepIndex(2, 1, 3)).toBeNull()
    expect(stepIndex(0, -1, 3)).toBeNull()
    expect(stepIndex(0, 1, 1)).toBeNull()
  })
})

// Type-level guard: the exported shape is what the provider consumes.
it('AudioItem is structurally usable', () => {
  const item: AudioItem = trackToAudioItem(track, album, 'x')
  expect(Object.keys(item)).toContain('streamUrl')
})
