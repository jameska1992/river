import { describe, expect, it } from 'vitest'
import { imageUrl } from './imageUrl'

// imageUrl rewrites the TMDB size segment on image.tmdb.org URLs and passes
// everything else through untouched. Tests cover each input class: empty-ish
// values, data/blob URIs, unparseable strings, non-TMDB hosts, and the two
// TMDB variants.
describe('imageUrl', () => {
  it('returns undefined for null / undefined', () => {
    expect(imageUrl(null)).toBeUndefined()
    expect(imageUrl(undefined)).toBeUndefined()
  })

  it('returns undefined for empty / whitespace-only strings', () => {
    expect(imageUrl('')).toBeUndefined()
    expect(imageUrl('   ')).toBeUndefined()
  })

  it('passes data: and blob: URIs through unchanged (trimmed)', () => {
    expect(imageUrl('data:image/png;base64,AAAA')).toBe('data:image/png;base64,AAAA')
    expect(imageUrl('blob:https://x/abc')).toBe('blob:https://x/abc')
    expect(imageUrl('  data:image/png;base64,AAAA  ')).toBe('data:image/png;base64,AAAA')
  })

  it('returns an unparseable value trimmed and unchanged', () => {
    expect(imageUrl('not a url')).toBe('not a url')
    expect(imageUrl('  /local/relative.jpg  ')).toBe('/local/relative.jpg')
  })

  it('passes non-TMDB hosts through unchanged', () => {
    const openlib = 'https://covers.openlibrary.org/b/id/123-L.jpg'
    expect(imageUrl(openlib)).toBe(openlib)
  })

  it('rewrites a TMDB poster to w342 by default', () => {
    expect(imageUrl('https://image.tmdb.org/t/p/original/abc.jpg')).toBe(
      'https://image.tmdb.org/t/p/w342/abc.jpg',
    )
  })

  it('rewrites a TMDB poster to w342 when variant is explicit', () => {
    expect(imageUrl('https://image.tmdb.org/t/p/w780/abc.jpg', 'poster')).toBe(
      'https://image.tmdb.org/t/p/w342/abc.jpg',
    )
  })

  it('rewrites a TMDB backdrop to w1280', () => {
    expect(imageUrl('https://image.tmdb.org/t/p/original/bd.jpg', 'backdrop')).toBe(
      'https://image.tmdb.org/t/p/w1280/bd.jpg',
    )
  })

  it('trims surrounding whitespace before rewriting a TMDB URL', () => {
    expect(imageUrl('  https://image.tmdb.org/t/p/original/abc.jpg  ')).toBe(
      'https://image.tmdb.org/t/p/w342/abc.jpg',
    )
  })

  it('leaves a TMDB URL without a /t/p/<size>/ segment unchanged', () => {
    const odd = 'https://image.tmdb.org/other/path.jpg'
    expect(imageUrl(odd)).toBe(odd)
  })
})
