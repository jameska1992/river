import { describe, expect, it, vi } from 'vitest'
import { MediabunnyEngine } from './MediabunnyEngine'
import { detectWebCodecs, probeRiverCodecs } from './capabilities'
import { movieSource } from '../riverSource'

// jsdom has no WebCodecs, so these cover the engine's capability gating and
// lifecycle — the paths that decide fallback. Decode/render behaviour is
// verified against real media in a browser (see docs/mediabunny-player.md).

describe('detectWebCodecs', () => {
  it('reports what is missing', () => {
    expect(detectWebCodecs({})).toEqual({
      videoDecoder: false, audioDecoder: false, audioContext: false,
      missing: ['VideoDecoder', 'AudioDecoder', 'AudioContext'],
    })
    const fn = function () {}
    expect(detectWebCodecs({ VideoDecoder: fn, AudioDecoder: fn, AudioContext: fn }).missing).toEqual([])
  })

  it('probeRiverCodecs is false without WebCodecs', async () => {
    expect(await probeRiverCodecs()).toEqual({ h264: false, aac: false })
  })
})

describe('MediabunnyEngine', () => {
  const source = { mediaId: 'm', resolveUrl: () => '/x' }

  it('rejects with "unsupported" when WebCodecs is unavailable, so the player falls back', async () => {
    const e = new MediabunnyEngine()
    const err = await e.load(source).catch(x => x)
    expect(err.code).toBe('unsupported')
    expect(e.getState()).toMatchObject({ status: 'error', error: { code: 'unsupported', recoverable: false } })
  })

  it('advertises its capabilities', () => {
    expect(new MediabunnyEngine().capabilities).toEqual({ audioTrackSwitching: true, playbackRate: false, pictureInPicture: false })
  })

  it('mounts a canvas and removes it on destroy', () => {
    const e = new MediabunnyEngine()
    const c = document.createElement('div')
    e.attach(c)
    expect(c.querySelector('canvas')).not.toBeNull()
    e.destroy()
    e.destroy()
    expect(c.querySelector('canvas')).toBeNull()
  })

  it('refuses to load after destroy', async () => {
    const e = new MediabunnyEngine()
    e.destroy()
    await expect(e.load(source)).rejects.toThrow(/destroyed/)
  })

  it('transport calls are no-ops before load', async () => {
    const e = new MediabunnyEngine()
    await e.play()
    e.pause()
    await e.seek(10)
    await e.reload()
    e.setVolume(3)
    e.setMuted(true)
    e.setPlaybackRate(2)
    expect(e.getState()).toMatchObject({ status: 'idle', volume: 1, muted: true })
    expect(e.getDiagnostics().notes).toContain('Playback rate is not supported by the Mediabunny engine yet')
    await expect(e.setAudioTrack('1')).rejects.toThrow(/Unknown audio track/)
  })

  it('notifies subscribers', async () => {
    const e = new MediabunnyEngine()
    const l = vi.fn()
    e.subscribe(l)
    await e.load(source).catch(() => {})
    expect(l).toHaveBeenCalled()
  })
})

describe('movieSource', () => {
  it('builds URLs through the River client each time', async () => {
    let tok = 'a'
    const api = {
      movieStreamUrl: (id: string) => `/api/movies/${id}/stream?token=${tok}`,
      audioTrackStreamUrl: (id: string) => `/api/audio-tracks/${id}/stream?token=${tok}`,
      refreshStreamToken: vi.fn(async () => { tok = 'b' }),
    }
    const s = movieSource(api, 'm1', 'Film', [
      { id: 't1', language: 'eng', label: '', media_type: 'movie', media_id: 'm1', stream_index: 1 } as never,
    ])
    expect(s.resolveUrl()).toContain('token=a')
    await s.refreshAuth!()
    expect(s.resolveUrl()).toContain('token=b')
    expect(s.audioVariants![0]).toMatchObject({ id: 't1', label: 'eng' })
    expect(s.audioVariants![0].resolveUrl()).toBe('/api/audio-tracks/t1/stream?token=b')
  })
})
