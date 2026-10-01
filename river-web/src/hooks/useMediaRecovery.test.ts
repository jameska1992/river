import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createRef } from 'react'

const { mockApi } = vi.hoisted(() => ({
  mockApi: { refreshStreamToken: vi.fn() },
}))
vi.mock('../api', () => ({ api: mockApi }))

import { useMediaRecovery } from './useMediaRecovery'

// A fake media element capturing the one-shot loadedmetadata handler so the
// test can drive the "reload finished" moment itself.
function makeMedia(over: Partial<HTMLMediaElement> = {}) {
  let metaHandler: (() => void) | null = null
  const el = {
    currentTime: 0,
    paused: true,
    error: null as MediaError | null,
    play: vi.fn().mockResolvedValue(undefined),
    addEventListener: vi.fn((ev: string, cb: () => void) => {
      if (ev === 'loadedmetadata') metaHandler = cb
    }),
    removeEventListener: vi.fn(),
    ...over,
  }
  return {
    el: el as unknown as HTMLMediaElement,
    fireLoadedMetadata: () => metaHandler?.(),
  }
}

function mediaRef(el: HTMLMediaElement | null) {
  const ref = createRef<HTMLMediaElement>()
  ;(ref as { current: HTMLMediaElement | null }).current = el
  return ref
}

let now = 0

beforeEach(() => {
  vi.clearAllMocks()
  now = 1_000_000
  vi.spyOn(Date, 'now').mockImplementation(() => now)
  mockApi.refreshStreamToken.mockResolvedValue(undefined)
  // jsdom lacks MediaError; the onError guard reads MEDIA_ERR_ABORTED off it.
  if (typeof (globalThis as Record<string, unknown>).MediaError === 'undefined') {
    ;(globalThis as Record<string, unknown>).MediaError = { MEDIA_ERR_ABORTED: 1 }
  }
})
afterEach(() => {
  vi.restoreAllMocks()
})

describe('useMediaRecovery', () => {
  it('refreshes the token, reloads with a cache-buster, and restores position + play', async () => {
    const { el, fireLoadedMetadata } = makeMedia({ currentTime: 120, paused: false })
    const buildSrc = vi.fn(() => 'https://x/stream?token=t')
    const setSrc = vi.fn()
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), buildSrc, setSrc))

    await act(async () => { await result.current.recover() })

    expect(mockApi.refreshStreamToken).toHaveBeenCalled()
    expect(buildSrc).toHaveBeenCalled()
    expect(setSrc).toHaveBeenCalledTimes(1)
    expect(setSrc.mock.calls[0][0]).toMatch(/^https:\/\/x\/stream\?token=t&_r=\d+$/)

    // Simulate the element finishing its reload.
    act(() => { fireLoadedMetadata() })
    expect(el.currentTime).toBe(120)
    expect(el.play).toHaveBeenCalled()
    expect(result.current.recoveringRef.current).toBe(false)
  })

  it('does not restore play-state when the element was paused', async () => {
    const { el, fireLoadedMetadata } = makeMedia({ currentTime: 30, paused: true })
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), () => 'https://x/s', vi.fn()))

    await act(async () => { await result.current.recover() })
    act(() => { fireLoadedMetadata() })

    expect(el.currentTime).toBe(30)
    expect(el.play).not.toHaveBeenCalled()
  })

  it('uses ? as the separator when the src has no query string', async () => {
    const { el } = makeMedia()
    const setSrc = vi.fn()
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), () => 'https://x/stream', setSrc))

    await act(async () => { await result.current.recover() })
    expect(setSrc.mock.calls[0][0]).toMatch(/^https:\/\/x\/stream\?_r=\d+$/)
  })

  it('is a no-op within the cooldown window', async () => {
    const { el } = makeMedia()
    const setSrc = vi.fn()
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), () => 'https://x/s', setSrc))

    await act(async () => { await result.current.recover() })
    expect(setSrc).toHaveBeenCalledTimes(1)

    // Finish the first reload so recoveringRef clears, then retry too soon.
    now += 1000 // < RECOVERY_COOLDOWN_MS (8000)
    await act(async () => { await result.current.recover() })
    expect(setSrc).toHaveBeenCalledTimes(1) // still just the one
  })

  it('recovers again after the cooldown elapses', async () => {
    const { el, fireLoadedMetadata } = makeMedia()
    const setSrc = vi.fn()
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), () => 'https://x/s', setSrc))

    await act(async () => { await result.current.recover() })
    act(() => { fireLoadedMetadata() }) // clears recoveringRef

    now += 9000 // past the cooldown
    await act(async () => { await result.current.recover() })
    expect(setSrc).toHaveBeenCalledTimes(2)
  })

  it('clears the recovering flag and does not reload when buildSrc returns nothing', async () => {
    const { el } = makeMedia()
    const setSrc = vi.fn()
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), () => undefined, setSrc))

    await act(async () => { await result.current.recover() })
    expect(setSrc).not.toHaveBeenCalled()
    expect(result.current.recoveringRef.current).toBe(false)
  })

  it('reloads anyway when the token refresh rejects', async () => {
    mockApi.refreshStreamToken.mockRejectedValue(new Error('offline'))
    const { el } = makeMedia()
    const setSrc = vi.fn()
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), () => 'https://x/s', setSrc))

    await act(async () => { await result.current.recover() })
    expect(setSrc).toHaveBeenCalledTimes(1)
  })

  it('does nothing when there is no media element', async () => {
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(null), () => 'https://x/s', vi.fn()))
    await act(async () => { await result.current.recover() })
    expect(mockApi.refreshStreamToken).not.toHaveBeenCalled()
  })

  it('onError ignores MEDIA_ERR_ABORTED (our own reload / navigation)', async () => {
    const aborted = { code: 1 } as MediaError // MEDIA_ERR_ABORTED
    const { el } = makeMedia({ error: aborted })
    const setSrc = vi.fn()
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), () => 'https://x/s', setSrc))

    await act(async () => { result.current.onError() })
    expect(setSrc).not.toHaveBeenCalled()
  })

  it('onError triggers recovery for a genuine stream error', async () => {
    const network = { code: 2 } as MediaError // MEDIA_ERR_NETWORK
    const { el } = makeMedia({ error: network })
    const setSrc = vi.fn()
    const { result } = renderHook(() =>
      useMediaRecovery(mediaRef(el), () => 'https://x/s', setSrc))

    await act(async () => { result.current.onError() })
    expect(setSrc).toHaveBeenCalledTimes(1)
  })
})

// The visibility listener recovers pre-emptively when a backgrounded tab / slept
// device returns and the timeline is stuck. These use fake timers to drive the
// 1.5s "did the timeline move?" probe.
describe('useMediaRecovery — visibility recovery', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(1_000_000) // past the cooldown so recover() can run
    mockApi.refreshStreamToken.mockResolvedValue(undefined)
    Object.defineProperty(document, 'visibilityState', {
      value: 'visible', configurable: true,
    })
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('recovers when the tab returns and the timeline has not advanced', async () => {
    const { el } = makeMedia({ paused: false, currentTime: 100 })
    const setSrc = vi.fn()
    renderHook(() => useMediaRecovery(mediaRef(el), () => 'https://x/s', setSrc))

    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(1500)

    expect(mockApi.refreshStreamToken).toHaveBeenCalled()
    expect(setSrc).toHaveBeenCalledTimes(1)
  })

  it('does not recover when the timeline advanced (playback is healthy)', async () => {
    const el = makeMedia({ paused: false, currentTime: 100 })
    const setSrc = vi.fn()
    renderHook(() => useMediaRecovery(mediaRef(el.el), () => 'https://x/s', setSrc))

    document.dispatchEvent(new Event('visibilitychange'))
    // The timeline moved on while the probe window was open.
    ;(el.el as unknown as { currentTime: number }).currentTime = 103
    await vi.advanceTimersByTimeAsync(1500)

    expect(setSrc).not.toHaveBeenCalled()
  })

  it('does not probe when the element is paused', async () => {
    const { el } = makeMedia({ paused: true, currentTime: 100 })
    const setSrc = vi.fn()
    renderHook(() => useMediaRecovery(mediaRef(el), () => 'https://x/s', setSrc))

    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(1500)

    expect(setSrc).not.toHaveBeenCalled()
  })

  it('ignores the event when the tab is not visible', async () => {
    Object.defineProperty(document, 'visibilityState', {
      value: 'hidden', configurable: true,
    })
    const { el } = makeMedia({ paused: false, currentTime: 100 })
    const setSrc = vi.fn()
    renderHook(() => useMediaRecovery(mediaRef(el), () => 'https://x/s', setSrc))

    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(1500)

    expect(setSrc).not.toHaveBeenCalled()
  })
})
