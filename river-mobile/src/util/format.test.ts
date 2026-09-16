import { describe, it, expect } from 'vitest'
import { detailPath, episodeCode, progressPct, formatDuration, initials } from './format'

describe('detailPath', () => {
  it('maps each media type to its route', () => {
    expect(detailPath('movie', '1')).toBe('/movies/1')
    expect(detailPath('tvshow', '2')).toBe('/tvshows/2')
    expect(detailPath('audiobook', '3')).toBe('/audiobooks/3')
    expect(detailPath('album', '4')).toBe('/albums/4')
    expect(detailPath('artist', '5')).toBe('/artists/5')
    expect(detailPath('unknown', '6')).toBe('/')
  })
})

describe('episodeCode', () => {
  it('zero-pads season and episode', () => {
    expect(episodeCode(2, 5)).toBe('S02E05')
    expect(episodeCode(10, 12)).toBe('S10E12')
  })
})

describe('progressPct', () => {
  it('computes a percentage', () => expect(progressPct(30, 120)).toBe(25))
  it('guards against zero/negative duration', () => {
    expect(progressPct(5, 0)).toBe(0)
    expect(progressPct(5, -1)).toBe(0)
  })
  it('clamps to 0–100', () => {
    expect(progressPct(200, 100)).toBe(100)
    expect(progressPct(-10, 100)).toBe(0)
  })
})

describe('formatDuration', () => {
  it('renders M:SS and H:MM:SS', () => {
    expect(formatDuration(65)).toBe('1:05')
    expect(formatDuration(3661)).toBe('1:01:01')
    expect(formatDuration(0)).toBe('0:00')
  })
})

describe('initials', () => {
  it('derives 1–2 letters', () => {
    expect(initials('The Beatles')).toBe('TB')
    expect(initials('alice')).toBe('AL')
    expect(initials('')).toBe('?')
    expect(initials('madonna')).toBe('MA')
  })
})
