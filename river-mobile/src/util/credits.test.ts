import { describe, it, expect } from 'vitest'
import { castToEntries, crewToEntries } from './credits'
import type { CastCredit, CrewCredit } from '../api'

const cast: CastCredit[] = [
  { person_id: 'p2', tmdb_id: null, name: 'Second Billed', profile_path: '/b.jpg', character: 'Villain', order: 1 },
  { person_id: 'p1', tmdb_id: null, name: 'Top Billed', profile_path: '', character: 'Hero', order: 0 },
]

const crew: CrewCredit[] = [
  { person_id: 'd1', tmdb_id: null, name: 'Jane Director', profile_path: '/j.jpg', job: 'Director', department: 'Directing' },
  { person_id: 'w1', tmdb_id: null, name: 'Will Writer', profile_path: '', job: 'Writer', department: 'Writing' },
  { person_id: 'd1', tmdb_id: null, name: 'Jane Director', profile_path: '/j.jpg', job: 'Producer', department: 'Production' },
]

describe('castToEntries', () => {
  it('sorts by billing order and maps character → role', () => {
    const e = castToEntries(cast)
    expect(e.map(x => x.name)).toEqual(['Top Billed', 'Second Billed'])
    expect(e[0].role).toBe('Hero')
    expect(e[0].imagePath).toBeUndefined() // empty profile_path drops out
    expect(e[1].imagePath).toBe('/b.jpg')
  })
})

describe('crewToEntries', () => {
  it('dedupes a person across jobs and joins the jobs', () => {
    const e = crewToEntries(crew)
    expect(e).toHaveLength(2)
    const jane = e.find(x => x.name === 'Jane Director')!
    expect(jane.role).toBe('Director, Producer')
    expect(jane.imagePath).toBe('/j.jpg')
  })
})
