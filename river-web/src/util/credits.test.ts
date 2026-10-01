import { describe, expect, it } from 'vitest'
import { dedupeCrew, type CrewEntry } from './credits'
import type { Credits, CrewCredit } from '../api'
import { creditsToRequest } from './credits'

// Build a CrewCredit touching only the fields dedupeCrew reads.
function crew(o: Partial<CrewCredit>): CrewCredit {
  return {
    person_id: '',
    tmdb_id: null,
    name: '',
    profile_path: '',
    job: '',
    department: '',
    ...o,
  }
}

describe('dedupeCrew', () => {
  it('returns an empty array for empty input', () => {
    expect(dedupeCrew([])).toEqual([])
  })

  it('keeps a single person with their job', () => {
    const out = dedupeCrew([crew({ person_id: 'p1', name: 'Ann', job: 'Director' })])
    expect(out).toEqual<CrewEntry[]>([
      { person_id: 'p1', name: 'Ann', profile_path: '', jobs: 'Director' },
    ])
  })

  it('joins distinct jobs for the same person', () => {
    const out = dedupeCrew([
      crew({ person_id: 'p1', name: 'Ann', job: 'Writer' }),
      crew({ person_id: 'p1', name: 'Ann', job: 'Producer' }),
    ])
    expect(out).toHaveLength(1)
    expect(out[0].jobs).toBe('Writer, Producer')
  })

  it('dedupes a repeated job for the same person', () => {
    const out = dedupeCrew([
      crew({ person_id: 'p1', name: 'Ann', job: 'Writer' }),
      crew({ person_id: 'p1', name: 'Ann', job: 'Writer' }),
    ])
    expect(out[0].jobs).toBe('Writer')
  })

  it('fills in the first job when the first-seen entry had an empty job', () => {
    const out = dedupeCrew([
      crew({ person_id: 'p1', name: 'Ann', job: '' }),
      crew({ person_id: 'p1', name: 'Ann', job: 'Editor' }),
    ])
    expect(out[0].jobs).toBe('Editor')
  })

  it('keys by name when a person has no id', () => {
    const out = dedupeCrew([
      crew({ person_id: '', name: 'Ann', job: 'Grip' }),
      crew({ person_id: '', name: 'Ann', job: 'Gaffer' }),
      crew({ person_id: '', name: 'Bob', job: 'Grip' }),
    ])
    expect(out.map(e => e.name)).toEqual(['Ann', 'Bob'])
    expect(out[0].jobs).toBe('Grip, Gaffer')
  })

  it('preserves first-seen order and carries profile_path', () => {
    const out = dedupeCrew([
      crew({ person_id: 'p2', name: 'Bob', profile_path: '/b.jpg', job: 'Composer' }),
      crew({ person_id: 'p1', name: 'Ann', profile_path: '/a.jpg', job: 'Director' }),
    ])
    expect(out.map(e => e.person_id)).toEqual(['p2', 'p1'])
    expect(out[0].profile_path).toBe('/b.jpg')
  })
})

describe('creditsToRequest', () => {
  const credits: Credits = {
    cast: [
      { person_id: 'c1', tmdb_id: 500, name: 'Lead', profile_path: '/l.jpg', character: 'Hero', order: 0 },
      { person_id: 'c2', tmdb_id: null, name: 'Extra', profile_path: '', character: 'Guard', order: 1 },
    ],
    crew: [
      { person_id: 'w1', tmdb_id: 42, name: 'Ann', profile_path: '/a.jpg', job: 'Director', department: 'Directing' },
      { person_id: 'w2', tmdb_id: null, name: 'Bob', profile_path: '', job: 'Gaffer', department: 'Lighting' },
    ],
  }

  it('maps cast fields and carries tmdb_id only when present', () => {
    const req = creditsToRequest(credits, false)
    expect(req.cast).toEqual([
      { tmdb_id: 500, name: 'Lead', profile_path: '/l.jpg', character: 'Hero', order: 0 },
      { name: 'Extra', profile_path: '', character: 'Guard', order: 1 },
    ])
    // the second entry omits tmdb_id entirely rather than sending null
    expect('tmdb_id' in req.cast[1]).toBe(false)
  })

  it('maps crew fields and carries tmdb_id only when present', () => {
    const req = creditsToRequest(credits, false)
    expect(req.crew).toEqual([
      { tmdb_id: 42, name: 'Ann', profile_path: '/a.jpg', job: 'Director', department: 'Directing' },
      { name: 'Bob', profile_path: '', job: 'Gaffer', department: 'Lighting' },
    ])
    expect('tmdb_id' in req.crew[1]).toBe(false)
  })

  it('passes the locked flag straight through', () => {
    expect(creditsToRequest(credits, true).locked).toBe(true)
    expect(creditsToRequest(credits, false).locked).toBe(false)
  })

  it('handles empty cast and crew', () => {
    const req = creditsToRequest({ cast: [], crew: [] }, true)
    expect(req).toEqual({ cast: [], crew: [], locked: true })
  })
})
