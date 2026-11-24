import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { NativeEngine } from './NativeEngine'
import type { PlaybackSource } from '../core/types'

// jsdom's HTMLMediaElement has no media pipeline, so the tests drive the
// element's events by hand and stub play()/pause().
function setup(source: Partial<PlaybackSource> = {}) {
  const engine = new NativeEngine()
  const v = engine.element
  let paused = true
  Object.defineProperty(v, 'paused', { get: () => paused, configurable: true })
  Object.defineProperty(v, 'duration', { value: 120, configurable: true, writable: true })
  v.play = vi.fn(async () => { paused = false; v.dispatchEvent(new Event('play')) })
  v.pause = vi.fn(() => { paused = true; v.dispatchEvent(new Event('pause')) })
  const container = document.createElement('div')
  engine.attach(container)
  const src: PlaybackSource = { mediaId: 'm', resolveUrl: () => '/api/movies/m/stream?token=a', ...source }
  const fire = (type: string) => v.dispatchEvent(new Event(type))
  return { engine, v, container, src, fire }
}

describe('NativeEngine', () => {
  beforeEach(() => { vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {}) })
  afterEach(() => { vi.restoreAllMocks() })

  it('mounts a <video> and loads the resolved URL', async () => {
    const { engine, v, container, src, fire } = setup()
    expect(container.querySelector('video')).toBe(v)
    const p = engine.load(src)
    expect(engine.getState().status).toBe('loading')
    expect(v.src).toContain('/api/movies/m/stream?token=a')
    fire('loadedmetadata')
    await p
    expect(engine.getState()).toMatchObject({ status: 'paused', duration: 120 })
  })

  it('applies startAt and autoplay once metadata loads', async () => {
    const { engine, v, src, fire } = setup()
    const p = engine.load(src, { startAt: 30, autoplay: true })
    fire('loadedmetadata')
    await p
    expect(v.currentTime).toBe(30)
    expect(v.play).toHaveBeenCalled()
    expect(engine.getState().status).toBe('playing')
  })

  it('rejects load on a media error', async () => {
    const { engine, src, fire } = setup()
    const p = engine.load(src)
    fire('error')
    await expect(p).rejects.toMatchObject({ code: 'network' })
  })

  it('maps element events to playback state', () => {
    const { engine, v, fire } = setup()
    fire('play')
    expect(engine.getState().status).toBe('playing')
    fire('waiting')
    expect(engine.getState().status).toBe('playing') // paused stub is true until play()
    v.currentTime = 12
    fire('timeupdate')
    expect(engine.getState().position).toBe(12)
    fire('ended')
    expect(engine.getState().status).toBe('ended')
    v.volume = 0.5
    v.muted = true
    fire('volumechange')
    expect(engine.getState()).toMatchObject({ volume: 0.5, muted: true })
  })

  it('reports buffering while playing', async () => {
    const { engine, fire } = setup()
    await engine.play()
    fire('waiting')
    expect(engine.getState().status).toBe('buffering')
  })

  it('seek resolves on seeked', async () => {
    const { engine, v, fire } = setup()
    const p = engine.seek(500)
    expect(engine.getState()).toMatchObject({ seeking: true, position: 120 })
    expect(v.currentTime).toBe(120)
    fire('seeked')
    await p
    expect(engine.getState().seeking).toBe(false)
  })

  it('controls volume, mute and rate on the element', () => {
    const { engine, v } = setup()
    engine.setVolume(2)
    expect(v.volume).toBe(1)
    engine.setMuted(true)
    expect(v.muted).toBe(true)
    engine.setPlaybackRate(1.5)
    expect(v.playbackRate).toBe(1.5)
  })

  it('switches audio by reloading onto a River variant file at the same position', async () => {
    const { engine, v, src, fire } = setup({
      audioVariants: [
        { id: 'a1', language: 'eng', label: 'English', resolveUrl: () => '/api/audio-tracks/a1/stream' },
        { id: 'a2', language: 'fre', label: '', resolveUrl: () => '/api/audio-tracks/a2/stream' },
      ],
    })
    const p = engine.load(src)
    fire('loadedmetadata')
    await p
    expect(engine.getState().audioTracks.map(t => t.label)).toEqual(['English', 'fre'])
    v.currentTime = 50
    const sw = engine.setAudioTrack('a2')
    expect(v.src).toContain('/api/audio-tracks/a2/stream')
    fire('loadedmetadata')
    await sw
    expect(v.currentTime).toBe(50)
    expect(engine.getState().activeAudioTrackId).toBe('a2')
    await expect(engine.setAudioTrack('nope')).rejects.toThrow(/Unknown/)
  })

  it('reload refreshes auth and cache-busts the current URL', async () => {
    const refreshAuth = vi.fn(async () => {})
    const { engine, v, src, fire } = setup({ refreshAuth })
    const p = engine.load(src)
    fire('loadedmetadata')
    await p
    const r = engine.reload()
    await Promise.resolve()
    await Promise.resolve()
    expect(refreshAuth).toHaveBeenCalled()
    expect(v.src).toMatch(/token=a&_r=\d+/)
    fire('loadedmetadata')
    await r
  })

  it('classifies element errors', () => {
    const { engine, v, fire } = setup()
    Object.defineProperty(v, 'error', { value: { code: 4, message: 'bad codec' }, configurable: true })
    fire('error')
    expect(engine.getState()).toMatchObject({ status: 'error', error: { code: 'unsupported' } })
  })

  it('treats SRC_NOT_SUPPORTED after a successful load as a recoverable network error (Firefox)', async () => {
    const { engine, v, src, fire } = setup()
    const p = engine.load(src)
    fire('loadedmetadata')
    await p
    Object.defineProperty(v, 'error', { value: { code: 4, message: 'offline' }, configurable: true })
    fire('error')
    expect(engine.getState().error).toMatchObject({ code: 'network', recoverable: true })
  })

  it('reload after an error restores the last healthy position and play state, not the reset element', async () => {
    const { engine, v, src, fire } = setup()
    const p = engine.load(src)
    fire('loadedmetadata')
    await p
    await engine.play()
    v.currentTime = 80
    fire('timeupdate')
    Object.defineProperty(v, 'error', { value: { code: 2, message: 'net' }, configurable: true })
    fire('error')
    // The broken element resets / pauses; none of that may leak into the resume point.
    v.currentTime = 0
    fire('timeupdate')
    v.pause()
    expect(engine.getState().position).toBe(80)
    const r = engine.reload()
    await Promise.resolve()
    ;(v.play as ReturnType<typeof vi.fn>).mockClear()
    fire('loadedmetadata')
    await r
    expect(v.currentTime).toBe(80)
    expect(v.play).toHaveBeenCalled()
  })

  it('ignores aborted errors from its own src swaps', () => {
    const { engine, v, fire } = setup()
    Object.defineProperty(v, 'error', { value: { code: 1, message: '' }, configurable: true })
    fire('error')
    expect(engine.getState().status).not.toBe('error')
  })

  it('destroy detaches the element and is idempotent', () => {
    const { engine, container } = setup()
    engine.destroy()
    engine.destroy()
    expect(container.querySelector('video')).toBeNull()
  })

  it('exposes diagnostics', () => {
    expect(setup().engine.getDiagnostics().sourceKind).toMatch(/HTMLVideoElement/)
  })
})
