import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { EngineError } from './core/PlaybackEngine'
import { PlaybackRecovery } from './core/PlaybackRecovery'
import { engineOrder, loadWithFallback } from './core/selectEngine'
import type { EngineKind, PlaybackSource } from './core/types'
import { FakeEngine } from './testing/FakeEngine'

vi.mock('./mediabunny/MediabunnyEngine', async () => {
  const { FakeEngine } = await import('./testing/FakeEngine')
  return { MediabunnyEngine: class extends FakeEngine { constructor() { super('mediabunny') } } }
})
vi.mock('./native/NativeEngine', async () => {
  const { FakeEngine } = await import('./testing/FakeEngine')
  return { NativeEngine: class extends FakeEngine { constructor() { super('native') } } }
})
vi.mock('./mediabunny/capabilities', () => ({
  detectWebCodecs: () => ({ videoDecoder: true, audioDecoder: true, audioContext: true, missing: [] }),
  probeRiverCodecs: async () => ({ h264: true, aac: true }),
}))

import { Player } from './react/Player'
import { DebugPanel } from './react/DebugPanel'

const source: PlaybackSource = { mediaId: 'm1', resolveUrl: () => '/api/movies/m1/stream?token=t' }

beforeEach(() => {
  FakeEngine.instances = []
  FakeEngine.loadBehaviour = {}
})

describe('engineOrder', () => {
  it('prefers Mediabunny with native fallback when WebCodecs exists', () => {
    expect(engineOrder('auto', true)).toEqual(['mediabunny', 'native'])
    expect(engineOrder('auto', false)).toEqual(['native'])
  })
  it('honours a forced engine', () => {
    expect(engineOrder('native', true)).toEqual(['native'])
    expect(engineOrder('mediabunny', false)).toEqual(['mediabunny'])
  })
})

describe('loadWithFallback', () => {
  const factory = (k: EngineKind) => new FakeEngine(k)
  const container = () => document.createElement('div')

  it('uses the first engine that loads', async () => {
    const sel = await loadWithFallback(['mediabunny', 'native'], factory, container(), source, {})
    expect(sel.engine.kind).toBe('mediabunny')
    expect(sel.reason).toBe('preferred')
  })

  it('falls back when an engine reports unsupported, destroying it', async () => {
    FakeEngine.loadBehaviour.mediabunny = async () => { throw new EngineError('unsupported', 'no H.264') }
    const seen: string[] = []
    const sel = await loadWithFallback(['mediabunny', 'native'], factory, container(), source, {}, e => seen.push(e.kind))
    expect(sel.engine.kind).toBe('native')
    expect(sel.reason).toMatch(/fell back: mediabunny: no H.264/)
    expect(seen).toEqual(['mediabunny', 'native'])
    expect(FakeEngine.instances[0].destroyed).toBe(true)
  })

  it('does not fall back on other errors', async () => {
    FakeEngine.loadBehaviour.mediabunny = async () => { throw new EngineError('network', 'down') }
    const sel = await loadWithFallback(['mediabunny', 'native'], factory, container(), source, {})
    expect(sel.engine.kind).toBe('mediabunny')
    expect(sel.reason).toMatch(/failed: down/)
    expect(FakeEngine.instances).toHaveLength(1)
  })

  it('reports a forced single engine', async () => {
    const sel = await loadWithFallback(['native'], factory, container(), source, {})
    expect(sel.reason).toBe('forced')
  })

  it('keeps the last engine when everything is unsupported', async () => {
    FakeEngine.loadBehaviour.native = async () => { throw new EngineError('unsupported', 'nope') }
    const sel = await loadWithFallback(['native'], factory, container(), source, {})
    expect(sel.reason).toMatch(/failed/)
  })
})

describe('PlaybackRecovery', () => {
  afterEach(() => { vi.useRealTimers() })

  it('reloads on a recoverable error, respecting the cooldown', async () => {
    let now = 0
    const e = new FakeEngine('mediabunny')
    const onRecover = vi.fn()
    const r = new PlaybackRecovery(e, { now: () => now, cooldownMs: 1000, onRecover })
    e.set({ status: 'error', error: { code: 'network', message: 'drop', recoverable: true } })
    await Promise.resolve()
    expect(e.reload).toHaveBeenCalledTimes(1)
    expect(onRecover).toHaveBeenCalledWith('error: drop')

    e.set({ status: 'paused', error: null })
    e.set({ status: 'error', error: { code: 'network', message: 'again', recoverable: true } })
    expect(e.reload).toHaveBeenCalledTimes(1) // within cooldown

    now = 2000
    expect(await r.recover('manual')).toBe(true)
    expect(e.reload).toHaveBeenCalledTimes(2)
    r.dispose()
  })

  it('ignores unrecoverable errors', () => {
    const e = new FakeEngine('mediabunny')
    const r = new PlaybackRecovery(e)
    e.set({ status: 'error', error: { code: 'unsupported', message: 'x', recoverable: false } })
    expect(e.reload).not.toHaveBeenCalled()
    r.dispose()
  })

  it('does not stack reloads', async () => {
    const e = new FakeEngine('mediabunny')
    let finish!: () => void
    e.reload.mockImplementation(() => new Promise<void>(res => { finish = res }))
    const r = new PlaybackRecovery(e, { cooldownMs: 0 })
    const first = r.recover('a')
    expect(await r.recover('b')).toBe(false)
    finish()
    expect(await first).toBe(true)
    r.dispose()
  })

  it('returns false when reload throws', async () => {
    const e = new FakeEngine('mediabunny')
    e.reload.mockRejectedValue(new Error('x'))
    const r = new PlaybackRecovery(e)
    expect(await r.recover('a')).toBe(false)
    r.dispose()
  })

  it('keeps retrying with backoff while the error persists', async () => {
    vi.useFakeTimers()
    let now = 0
    const e = new FakeEngine('mediabunny')
    // Every reload fails and leaves the engine in a recoverable error.
    e.reload.mockImplementation(async () => {
      e.set({ status: 'loading', error: null })
      e.set({ status: 'error', error: { code: 'network', message: 'offline', recoverable: true } })
      throw new Error('offline')
    })
    const r = new PlaybackRecovery(e, { now: () => now, cooldownMs: 1000, maxBackoffMs: 4000 })
    e.set({ status: 'error', error: { code: 'network', message: 'drop', recoverable: true } })
    await vi.advanceTimersByTimeAsync(0)
    expect(e.reload).toHaveBeenCalledTimes(1)
    // Cooldown doubled to 2s after the failure.
    now = 1500; await vi.advanceTimersByTimeAsync(1500)
    expect(e.reload).toHaveBeenCalledTimes(1)
    now = 2000; await vi.advanceTimersByTimeAsync(500)
    expect(e.reload).toHaveBeenCalledTimes(2)
    // Capped at 4s.
    now = 6000; await vi.advanceTimersByTimeAsync(4000)
    expect(e.reload).toHaveBeenCalledTimes(3)
    r.dispose()
  })

  it('recovers immediately when the browser comes back online', async () => {
    const e = new FakeEngine('mediabunny')
    const r = new PlaybackRecovery(e, { cooldownMs: 60000 })
    await r.recover('first')
    e.set({ status: 'error', error: { code: 'network', message: 'offline', recoverable: true } })
    expect(e.reload).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new Event('online'))
    await Promise.resolve()
    expect(e.reload).toHaveBeenCalledTimes(2)
    r.dispose()
  })

  it('reloads when playback is stuck after the tab becomes visible', () => {
    vi.useFakeTimers()
    const e = new FakeEngine('mediabunny')
    const r = new PlaybackRecovery(e)
    e.set({ status: 'playing', position: 42 })
    document.dispatchEvent(new Event('visibilitychange'))
    vi.advanceTimersByTime(1600)
    expect(e.reload).toHaveBeenCalledTimes(1)
    r.dispose()
  })

  it('does not reload if the position advances after becoming visible', () => {
    vi.useFakeTimers()
    const e = new FakeEngine('mediabunny')
    const r = new PlaybackRecovery(e)
    e.set({ status: 'playing', position: 42 })
    document.dispatchEvent(new Event('visibilitychange'))
    e.set({ position: 43 })
    vi.advanceTimersByTime(1600)
    expect(e.reload).not.toHaveBeenCalled()
    r.dispose()
  })
})

describe('<Player>', () => {
  async function renderPlayer(props: Partial<React.ComponentProps<typeof Player>> = {}) {
    const onEngine = vi.fn()
    const utils = render(<Player source={source} onEngine={onEngine} {...props} />)
    await waitFor(() => expect(onEngine).toHaveBeenCalledWith(expect.objectContaining({ reason: expect.any(String) })))
    const engine = FakeEngine.instances.at(-1)!
    return { ...utils, engine, onEngine }
  }

  it('loads the preferred engine into its surface', async () => {
    const { engine } = await renderPlayer()
    expect(engine.kind).toBe('mediabunny')
    expect(engine.attached).toBeInstanceOf(HTMLDivElement)
    expect(screen.getByText('0:00 / 1:40')).toBeInTheDocument()
  })

  it('falls back to native when Mediabunny is unsupported', async () => {
    FakeEngine.loadBehaviour.mediabunny = async () => { throw new EngineError('unsupported', 'x') }
    const { engine } = await renderPlayer()
    expect(engine.kind).toBe('native')
  })

  it('drives play/pause, skip and mute through the engine', async () => {
    const { engine } = await renderPlayer()
    fireEvent.click(screen.getByLabelText('Play'))
    expect(engine.play).toHaveBeenCalled()
    await screen.findByLabelText('Pause')
    fireEvent.click(screen.getByLabelText('Pause'))
    expect(engine.pause).toHaveBeenCalled()
    fireEvent.click(screen.getByLabelText('Forward 10 seconds'))
    expect(engine.seek).toHaveBeenCalledWith(10)
    fireEvent.click(screen.getByLabelText('Mute'))
    expect(engine.setMuted).toHaveBeenCalledWith(true)
  })

  it('handles keyboard shortcuts', async () => {
    const { engine } = await renderPlayer()
    act(() => { engine.set({ position: 50 }) })
    fireEvent.keyDown(document, { key: ' ' })
    expect(engine.play).toHaveBeenCalled()
    fireEvent.keyDown(document, { key: 'ArrowLeft' })
    expect(engine.seek).toHaveBeenCalledWith(40)
    fireEvent.keyDown(document, { key: 'm' })
    expect(engine.setMuted).toHaveBeenCalledWith(true)
  })

  it('commits a scrub only on release', async () => {
    const { engine } = await renderPlayer()
    const seek = screen.getByLabelText('Seek')
    fireEvent.change(seek, { target: { value: '30' } })
    expect(engine.seek).not.toHaveBeenCalled()
    expect(screen.getByText('0:30 / 1:40')).toBeInTheDocument()
    fireEvent.pointerUp(seek)
    expect(engine.seek).toHaveBeenCalledWith(30)
  })

  it('sets volume and treats zero as muted', async () => {
    const { engine } = await renderPlayer()
    fireEvent.change(screen.getByLabelText('Volume'), { target: { value: '0' } })
    expect(engine.setVolume).toHaveBeenCalledWith(0)
    expect(engine.setMuted).toHaveBeenCalledWith(true)
  })

  it('shows loading/buffering and error states with retry', async () => {
    const { engine } = await renderPlayer()
    act(() => { engine.set({ status: 'buffering' }) })
    expect(screen.getByRole('status', { name: 'Buffering' })).toBeInTheDocument()
    act(() => { engine.set({ status: 'error', error: { code: 'network', message: 'Stream lost', recoverable: false } }) })
    expect(screen.getByRole('alert')).toHaveTextContent('Stream lost')
    fireEvent.click(screen.getByText('Retry'))
    expect(engine.reload).toHaveBeenCalled()
  })

  it('offers audio tracks only when there is a choice', async () => {
    const { engine } = await renderPlayer()
    expect(screen.queryByLabelText('Audio track')).not.toBeInTheDocument()
    act(() => {
      engine.set({
        audioTracks: [{ id: '2', type: 'audio', label: 'English' }, { id: '3', type: 'audio', label: 'French' }],
        activeAudioTrackId: '2',
      })
    })
    fireEvent.change(screen.getByLabelText('Audio track'), { target: { value: '3' } })
    expect(engine.setAudioTrack).toHaveBeenCalledWith('3')
  })

  it('surfaces a failed action', async () => {
    const { engine } = await renderPlayer()
    engine.play.mockRejectedValueOnce(new Error('Audio output is blocked'))
    fireEvent.click(screen.getByLabelText('Play'))
    expect(await screen.findByRole('alert')).toHaveTextContent('Audio output is blocked')
  })

  it('destroys the engine on unmount and source change', async () => {
    const { engine, rerender, onEngine } = await renderPlayer()
    rerender(<Player source={{ ...source }} onEngine={onEngine} />)
    expect(engine.destroyed).toBe(true)
    await waitFor(() => expect(FakeEngine.instances).toHaveLength(2))
  })

  it('renders the debug panel from engine state and diagnostics', async () => {
    const { engine } = await renderPlayer()
    act(() => {
      engine.set({ videoTrack: { id: '1', type: 'video', label: 'v', codec: 'avc', width: 1920, height: 1080 } })
    })
    render(<DebugPanel engine={engine} reason="preferred" />)
    expect(screen.getByText('mediabunny (preferred)')).toBeInTheDocument()
    expect(screen.getByText(/AVC · 1920×1080/)).toBeInTheDocument()
    expect(screen.getByText('fake')).toBeInTheDocument()
    expect(await screen.findByText('yes / yes')).toBeInTheDocument()
  })
})
