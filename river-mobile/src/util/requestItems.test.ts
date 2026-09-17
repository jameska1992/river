import { describe, it, expect } from 'vitest'
import { requestItems } from './requestItems'
import type { MovieSearchResult, ShowSearchResult } from '../api'

const movies: MovieSearchResult[] = [
  { tmdbId: 603, title: 'The Matrix', year: 1999, overview: 'Neo…', poster: '/m.jpg', added: false },
]
const shows: ShowSearchResult[] = [
  { tvdbId: 121361, title: 'Game of Thrones', year: 2011, overview: 'Westeros…', poster: '/s.jpg', added: true },
]

describe('requestItems', () => {
  it('maps movies keyed on tmdbId', () => {
    const [m] = requestItems('movies', movies, shows)
    expect(m).toMatchObject({ id: 603, title: 'The Matrix', year: 1999, added: false, kind: 'movies' })
  })

  it('maps shows keyed on tvdbId', () => {
    const [s] = requestItems('shows', movies, shows)
    expect(s).toMatchObject({ id: 121361, title: 'Game of Thrones', added: true, kind: 'shows' })
  })

  it('returns only the selected tab and empty when it has none', () => {
    expect(requestItems('movies', [], shows)).toEqual([])
    expect(requestItems('shows', movies, [])).toEqual([])
  })
})
