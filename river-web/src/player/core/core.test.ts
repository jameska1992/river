import { describe, expect, it, vi } from 'vitest'
import { PlaybackClock } from './PlaybackClock'
import { PlaybackStore } from './PlaybackStore'
import { EngineError, toPlaybackError } from './PlaybackEngine'
import { bufferedAhead, clampSeek, formatTime, fromTimeRanges } from './time'

describe('PlaybackStore', () => {
  it('notifies subscribers only on real changes', () => {
    const s = new PlaybackStore()
    const l = vi.fn()
    s.subscribe(l)
    s.update({ status: 'loading' })
    s.update({ status: 'loading' })
    expect(l).toHaveBeenCalledTimes(1)
    expect(s.getState().status).toBe('loading')
  })

  it('replaces state immutably', () => {
    const s = new PlaybackStore()
    const before = s.getState()
    s.update({ position: 5 })
    expect(s.getState()).not.toBe(before)
    expect(before.position).toBe(0)
  })

  it('reset keeps volume/mute preferences', () => {
    const s = new PlaybackStore()
    s.update({ volume: 0.3, muted: true, position: 99, status: 'playing' })
    s.reset({ status: 'loading' })
    expect(s.getState()).toMatchObject({ volume: 0.3, muted: true, position: 0, status: 'loading' })
  })

  it('unsubscribe and clear stop notifications', () => {
    const s = new PlaybackStore()
    const a = vi.fn()
    const b = vi.fn()
    const off = s.subscribe(a)
    s.subscribe(b)
    off()
    s.update({ position: 1 })
    expect(a).not.toHaveBeenCalled()
    expect(b).toHaveBeenCalledTimes(1)
    s.clear()
    s.update({ position: 2 })
    expect(b).toHaveBeenCalledTimes(1)
  })
})

describe('PlaybackClock', () => {
  function clock() {
    let now = 100
    const c = new PlaybackClock(() => now)
    return { c, advance: (s: number) => { now += s } }
  }

  it('stays frozen until started', () => {
    const { c, advance } = clock()
    advance(5)
    expect(c.getTime()).toBe(0)
    expect(c.isRunning).toBe(false)
  })

  it('advances with the time source while running and freezes on stop', () => {
    const { c, advance } = clock()
    c.start()
    advance(2.5)
    expect(c.getTime()).toBeCloseTo(2.5)
    c.stop()
    advance(10)
    expect(c.getTime()).toBeCloseTo(2.5)
    c.start()
    advance(1)
    expect(c.getTime()).toBeCloseTo(3.5)
  })

  it('set() jumps while running or stopped', () => {
    const { c, advance } = clock()
    c.set(60)
    expect(c.getTime()).toBe(60)
    c.start()
    advance(1)
    c.set(10)
    advance(1)
    expect(c.getTime()).toBeCloseTo(11)
  })

  it('clamps to [0, duration]', () => {
    const { c, advance } = clock()
    c.setDuration(5)
    c.set(-3)
    expect(c.getTime()).toBe(0)
    c.start()
    advance(100)
    expect(c.getTime()).toBe(5)
    c.set(50)
    expect(c.getTime()).toBe(5)
  })

  it('maps media time onto the source timeline for audio scheduling', () => {
    const { c, advance } = clock()
    c.set(30)
    c.start() // source 100 ↔ media 30
    advance(1)
    expect(c.toSourceTime(31)).toBeCloseTo(101)
    expect(c.toSourceTime(33)).toBeCloseTo(103)
  })

  it('start/stop are idempotent', () => {
    const { c, advance } = clock()
    c.start()
    advance(1)
    c.start()
    advance(1)
    expect(c.getTime()).toBeCloseTo(2)
    c.stop()
    c.stop()
    expect(c.getTime()).toBeCloseTo(2)
  })
})

describe('time helpers', () => {
  it('formatTime', () => {
    expect(formatTime(0)).toBe('0:00')
    expect(formatTime(65.9)).toBe('1:05')
    expect(formatTime(3600 + 62)).toBe('1:01:02')
    expect(formatTime(NaN)).toBe('0:00')
    expect(formatTime(-1)).toBe('0:00')
  })

  it('clampSeek', () => {
    expect(clampSeek(50, 100)).toBe(50)
    expect(clampSeek(-5, 100)).toBe(0)
    expect(clampSeek(500, 100)).toBe(100)
    expect(clampSeek(NaN, 100)).toBe(0)
    expect(clampSeek(42, 0)).toBe(42)
  })

  it('bufferedAhead finds the range containing the position', () => {
    const ranges = [{ start: 0, end: 10 }, { start: 20, end: 35 }]
    expect(bufferedAhead(ranges, 5)).toBe(5)
    expect(bufferedAhead(ranges, 25)).toBe(10)
    expect(bufferedAhead(ranges, 15)).toBe(0)
    expect(bufferedAhead(ranges, 19.95)).toBeCloseTo(15.05)
  })

  it('fromTimeRanges converts DOM TimeRanges', () => {
    const tr = { length: 2, start: (i: number) => [0, 5][i], end: (i: number) => [2, 9][i] } as TimeRanges
    expect(fromTimeRanges(tr)).toEqual([{ start: 0, end: 2 }, { start: 5, end: 9 }])
    expect(fromTimeRanges(null)).toEqual([])
  })
})

describe('errors', () => {
  it('EngineError defaults recoverability by code', () => {
    expect(new EngineError('network', 'x').recoverable).toBe(true)
    expect(new EngineError('decode', 'x').recoverable).toBe(true)
    expect(new EngineError('unsupported', 'x').recoverable).toBe(false)
    expect(new EngineError('network', 'x', { recoverable: false }).recoverable).toBe(false)
  })

  it('toPlaybackError normalises anything', () => {
    expect(toPlaybackError(new EngineError('auth', 'nope'))).toMatchObject({ code: 'auth', message: 'nope', recoverable: false })
    expect(toPlaybackError(new Error('boom'), 'network')).toMatchObject({ code: 'network', message: 'boom', recoverable: true })
    expect(toPlaybackError('str')).toMatchObject({ code: 'unknown', message: 'str', recoverable: false })
  })
})
