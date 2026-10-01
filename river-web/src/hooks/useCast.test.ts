import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useCast } from './useCast'

// The hook talks to the Google Cast SDK through two bare globals, `cast` and
// `chrome`, that the real app loads from an external <script>. We install
// minimal fakes on globalThis and drive the session-state listener by hand.

type SessionListener = () => void

interface CastMocks {
  setOptions: ReturnType<typeof vi.fn>
  addEventListener: ReturnType<typeof vi.fn>
  getSessionState: ReturnType<typeof vi.fn>
  getCurrentSession: ReturnType<typeof vi.fn>
  fireSessionChange: () => void
  loadMedia: ReturnType<typeof vi.fn>
}

const SESSION = {
  NO_SESSION: 'none',
  SESSION_STARTED: 'started',
  SESSION_RESUMED: 'resumed',
}

function installCast(opts: { available: boolean }): CastMocks {
  const setOptions = vi.fn()
  const addEventListener = vi.fn()
  const getSessionState = vi.fn().mockReturnValue(SESSION.NO_SESSION)
  const loadMedia = vi.fn().mockResolvedValue(undefined)
  const getCurrentSession = vi.fn().mockReturnValue(null)

  let sessionListener: SessionListener | null = null
  addEventListener.mockImplementation((_type: string, cb: SessionListener) => {
    sessionListener = cb
  })

  const ctx = { setOptions, addEventListener, getSessionState, getCurrentSession }

  const castGlobal = {
    framework: {
      CastContext: { getInstance: () => ctx },
      SessionState: SESSION,
      CastContextEventType: { SESSION_STATE_CHANGED: 'sessionstatechanged' },
    },
  }

  const chromeGlobal = {
    cast: {
      media: {
        DEFAULT_MEDIA_RECEIVER_APP_ID: 'CC1AD845',
        MediaInfo: class {
          metadata: unknown = null
          contentUrl: string
          contentType: string
          constructor(contentUrl: string, contentType: string) {
            this.contentUrl = contentUrl
            this.contentType = contentType
          }
        },
        LoadRequest: class {
          currentTime?: number
          media: unknown
          constructor(media: unknown) { this.media = media }
        },
      },
      AutoJoinPolicy: { ORIGIN_SCOPED: 'origin_scoped' },
    },
  }

  const g = globalThis as Record<string, unknown>
  g.cast = opts.available ? castGlobal : undefined
  g.chrome = chromeGlobal
  // The SDK's ready callback the hook registers when `cast` isn't up yet.
  ;(window as unknown as { __onGCastApiAvailable?: unknown }).__onGCastApiAvailable = undefined

  return {
    setOptions, addEventListener, getSessionState, getCurrentSession, loadMedia,
    fireSessionChange: () => sessionListener?.(),
  }
}

afterEach(() => {
  const g = globalThis as Record<string, unknown>
  delete g.cast
  delete g.chrome
  vi.restoreAllMocks()
})

describe('useCast', () => {
  it('initialises immediately when the Cast SDK is already present', async () => {
    const m = installCast({ available: true })
    const { result } = renderHook(() => useCast())

    await waitFor(() => expect(result.current.castReady).toBe(true))
    expect(m.setOptions).toHaveBeenCalledWith({
      receiverApplicationId: 'CC1AD845',
      autoJoinPolicy: 'origin_scoped',
    })
    expect(result.current.isCasting).toBe(false)
  })

  it('reflects an active session via the session-state listener', async () => {
    const m = installCast({ available: true })
    const { result } = renderHook(() => useCast())
    await waitFor(() => expect(result.current.castReady).toBe(true))

    m.getSessionState.mockReturnValue(SESSION.SESSION_STARTED)
    act(() => { m.fireSessionChange() })
    expect(result.current.isCasting).toBe(true)

    m.getSessionState.mockReturnValue(SESSION.NO_SESSION)
    act(() => { m.fireSessionChange() })
    expect(result.current.isCasting).toBe(false)
  })

  it('defers initialisation until __onGCastApiAvailable fires', async () => {
    const m = installCast({ available: false })
    const { result } = renderHook(() => useCast())

    // Not ready yet — the SDK hasn't loaded.
    expect(result.current.castReady).toBe(false)
    expect(m.setOptions).not.toHaveBeenCalled()

    // The SDK loads: publish `cast` and invoke the registered callback.
    ;(globalThis as Record<string, unknown>).cast = {
      framework: {
        CastContext: { getInstance: () => ({
          setOptions: m.setOptions,
          addEventListener: m.addEventListener,
          getSessionState: m.getSessionState,
          getCurrentSession: m.getCurrentSession,
        }) },
        SessionState: SESSION,
        CastContextEventType: { SESSION_STATE_CHANGED: 'sessionstatechanged' },
      },
    }
    const cb = (window as unknown as { __onGCastApiAvailable: (a: boolean) => void }).__onGCastApiAvailable
    act(() => { cb(true) })

    await waitFor(() => expect(result.current.castReady).toBe(true))
    expect(m.setOptions).toHaveBeenCalled()
  })

  it('loadCastMedia is a no-op when there is no current session', () => {
    const m = installCast({ available: true })
    m.getCurrentSession.mockReturnValue(null)
    const { result } = renderHook(() => useCast())

    result.current.loadCastMedia('https://x/stream', 'video/mp4', { title: 't' })
    expect(m.loadMedia).not.toHaveBeenCalled()
  })

  it('loadCastMedia builds a LoadRequest and sends it on the active session', () => {
    const m = installCast({ available: true })
    const session = { loadMedia: m.loadMedia }
    m.getCurrentSession.mockReturnValue(session)
    const { result } = renderHook(() => useCast())

    result.current.loadCastMedia('https://x/stream', 'video/mp4', { title: 't' }, 42)

    expect(m.loadMedia).toHaveBeenCalledTimes(1)
    const request = m.loadMedia.mock.calls[0][0] as { media: { contentUrl: string; contentType: string; metadata: unknown }; currentTime?: number }
    expect(request.media.contentUrl).toBe('https://x/stream')
    expect(request.media.contentType).toBe('video/mp4')
    expect(request.media.metadata).toEqual({ title: 't' })
    expect(request.currentTime).toBe(42)
  })

  it('loadCastMedia omits currentTime when not advanced past 0', () => {
    const m = installCast({ available: true })
    m.getCurrentSession.mockReturnValue({ loadMedia: m.loadMedia })
    const { result } = renderHook(() => useCast())

    result.current.loadCastMedia('https://x/stream', 'video/mp4', null, 0)
    const request = m.loadMedia.mock.calls[0][0] as { currentTime?: number }
    expect(request.currentTime).toBeUndefined()
  })
})
