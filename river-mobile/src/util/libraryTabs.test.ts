import { describe, it, expect } from 'vitest'
import { libraryTabs } from './libraryTabs'

describe('libraryTabs', () => {
  it('shows a tab only for configured library types', () => {
    expect(libraryTabs(['movie', 'tvshow'])).toEqual(['movies', 'tvshows'])
    expect(libraryTabs(['audiobook'])).toEqual(['audiobooks'])
  })

  it('omits types with no configured library (the #153 bug)', () => {
    // Movies + books configured, no music library → no Music tab.
    expect(libraryTabs(['movie', 'audiobook'])).toEqual(['movies', 'audiobooks'])
  })

  it('keeps a stable order regardless of input order', () => {
    expect(libraryTabs(['audiobook', 'music', 'movie', 'tvshow']))
      .toEqual(['movies', 'tvshows', 'music', 'audiobooks'])
  })

  it('dedupes multiple libraries of the same type', () => {
    expect(libraryTabs(['movie', 'movie', 'tvshow'])).toEqual(['movies', 'tvshows'])
  })

  it('returns nothing when no libraries are configured', () => {
    expect(libraryTabs([])).toEqual([])
  })
})
