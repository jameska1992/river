import { describe, it, expect } from 'vitest'
import { shouldResume, clampSeek, seekFraction, gestureSide, gestureValue, isVerticalDrag } from './player'

describe('shouldResume', () => {
  it('resumes when meaningfully into the file', () => {
    expect(shouldResume(120, 3600)).toBe(true)
  })
  it('does not resume from the very start', () => {
    expect(shouldResume(3, 3600)).toBe(false)
    expect(shouldResume(0, 3600)).toBe(false)
  })
  it('does not resume within the last 30s', () => {
    expect(shouldResume(3595, 3600)).toBe(false)
  })
  it('does not resume without a known duration', () => {
    expect(shouldResume(120, 0)).toBe(false)
  })
})

describe('clampSeek', () => {
  it('adds the delta within bounds', () => {
    expect(clampSeek(100, 10, 200)).toBe(110)
    expect(clampSeek(100, -10, 200)).toBe(90)
  })
  it('clamps at zero and duration', () => {
    expect(clampSeek(5, -10, 200)).toBe(0)
    expect(clampSeek(195, 10, 200)).toBe(200)
  })
  it('treats an unknown duration as zero ceiling', () => {
    expect(clampSeek(0, 10, 0)).toBe(0)
  })
})

describe('seekFraction', () => {
  it('maps a pointer position to 0–1 along the bar', () => {
    expect(seekFraction(150, 100, 200)).toBe(0.25)
    expect(seekFraction(200, 100, 200)).toBe(0.5)
  })
  it('saturates past either end', () => {
    expect(seekFraction(50, 100, 200)).toBe(0)
    expect(seekFraction(400, 100, 200)).toBe(1)
  })
  it('guards a zero-width bar', () => {
    expect(seekFraction(150, 100, 0)).toBe(0)
  })
})

describe('gestureSide', () => {
  it('left half = volume, right half = brightness', () => {
    expect(gestureSide(50, 400)).toBe('volume')
    expect(gestureSide(300, 400)).toBe('brightness')
  })
})

describe('gestureValue', () => {
  it('raises the value when dragging up, lowers when dragging down', () => {
    expect(gestureValue(0.5, 100, 200)).toBe(1)     // +0.5
    expect(gestureValue(0.5, -100, 200)).toBe(0)    // -0.5
    expect(gestureValue(0.5, 40, 200)).toBeCloseTo(0.7)
  })
  it('clamps to 0–1 and guards a zero span', () => {
    expect(gestureValue(0.9, 1000, 200)).toBe(1)
    expect(gestureValue(0.1, -1000, 200)).toBe(0)
    expect(gestureValue(0.5, 100, 0)).toBe(0.5)
  })
})

describe('isVerticalDrag', () => {
  it('is true for a mostly-vertical move past the dead-zone', () => {
    expect(isVerticalDrag(3, 40)).toBe(true)
  })
  it('is false for taps and horizontal swipes', () => {
    expect(isVerticalDrag(2, 4)).toBe(false)   // within dead-zone
    expect(isVerticalDrag(50, 20)).toBe(false) // more horizontal
  })
})
