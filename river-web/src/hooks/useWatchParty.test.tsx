import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRef } from 'react'
import type { WatchPartyMember } from '../api'

// Mock the api singleton (openWatchPartySocket) and react-router's useNavigate.
const { mockApi, navigate } = vi.hoisted(() => ({
  mockApi: { openWatchPartySocket: vi.fn() },
  navigate: vi.fn(),
}))
vi.mock('../api', () => ({ api: mockApi }))
vi.mock('react-router-dom', async (orig) => ({
  ...(await orig<typeof import('react-router-dom')>()),
  useNavigate: () => navigate,
}))

import { useWatchParty } from './useWatchParty'

// A fake socket whose received-message callback the test drives directly.
function makeSocket() {
  let onMsg: ((msg: unknown) => void) | null = null
  return {
    send: vi.fn(),
    close: vi.fn(),
    onMessage: vi.fn((cb: (msg: unknown) => void) => { onMsg = cb }),
    emit: (msg: unknown) => onMsg?.(msg),
  }
}

// A fake <video> that records currentTime / play / pause and is "ready".
function makeVideo(over: Partial<HTMLVideoElement> = {}) {
  const v = {
    currentTime: 0,
    paused: true,
    readyState: 4,
    play: vi.fn().mockResolvedValue(undefined),
    pause: vi.fn(function (this: { paused: boolean }) { this.paused = true }),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    ...over,
  }
  return v as unknown as HTMLVideoElement
}

function videoRef(v: HTMLVideoElement | null) {
  const ref = createRef<HTMLVideoElement>()
  ;(ref as { current: HTMLVideoElement | null }).current = v
  return ref
}

const members: WatchPartyMember[] = [
  { user_id: 'u1', username: 'alice' },
  { user_id: 'u2', username: 'bob' },
]

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useWatchParty', () => {
  it('does not open a socket without a roomId', () => {
    renderHook(() => useWatchParty(undefined, videoRef(makeVideo()), false, '/back'))
    expect(mockApi.openWatchPartySocket).not.toHaveBeenCalled()
  })

  it('opens a socket for the room and registers a message handler', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    renderHook(() => useWatchParty('room1', videoRef(makeVideo()), false, '/back'))

    expect(mockApi.openWatchPartySocket).toHaveBeenCalledWith('room1')
    expect(socket.onMessage).toHaveBeenCalled()
  })

  it('a "state" message marks connected and stores members', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    const { result } = renderHook(() =>
      useWatchParty('room1', videoRef(makeVideo()), true, '/back'))

    act(() => { socket.emit({ type: 'state', members, position: 0, playing: false }) })
    expect(result.current.connected).toBe(true)
    expect(result.current.members).toEqual(members)
  })

  it('a non-host syncs position and play-state from a "state" message', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    const video = makeVideo({ currentTime: 0, paused: true })
    renderHook(() => useWatchParty('room1', videoRef(video), false, '/back'))

    act(() => { socket.emit({ type: 'state', position: 50, playing: true }) })
    // > 2s gap → seeks, and playing → play()
    expect(video.currentTime).toBe(50)
    expect(video.play).toHaveBeenCalled()
  })

  it('a host ignores sync from a "state" message', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    const video = makeVideo({ currentTime: 0, paused: true })
    renderHook(() => useWatchParty('room1', videoRef(video), true, '/back'))

    act(() => { socket.emit({ type: 'state', position: 50, playing: true }) })
    expect(video.currentTime).toBe(0)
    expect(video.play).not.toHaveBeenCalled()
  })

  it('a "members" message updates the roster', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    const { result } = renderHook(() =>
      useWatchParty('room1', videoRef(makeVideo()), false, '/back'))

    act(() => { socket.emit({ type: 'members', members }) })
    expect(result.current.members).toEqual(members)
  })

  it('a non-host applies play / pause / seek commands', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    const video = makeVideo({ currentTime: 0, paused: true })
    renderHook(() => useWatchParty('room1', videoRef(video), false, '/back'))

    act(() => { socket.emit({ type: 'play', position: 12 }) })
    expect(video.currentTime).toBe(12)
    expect(video.play).toHaveBeenCalled()

    act(() => { socket.emit({ type: 'pause', position: 20 }) })
    expect(video.currentTime).toBe(20)
    expect(video.pause).toHaveBeenCalled()

    act(() => { socket.emit({ type: 'seek', position: 33 }) })
    expect(video.currentTime).toBe(33)
  })

  it('a "closed" message closes the socket and navigates to backPath', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    renderHook(() => useWatchParty('room1', videoRef(makeVideo()), false, '/back'))

    act(() => { socket.emit({ type: 'closed' }) })
    expect(socket.close).toHaveBeenCalled()
    expect(navigate).toHaveBeenCalledWith('/back', { replace: true })
  })

  it('sendCommand forwards to the socket', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    const { result } = renderHook(() =>
      useWatchParty('room1', videoRef(makeVideo()), true, '/back'))

    act(() => { result.current.sendCommand('seek', 99) })
    expect(socket.send).toHaveBeenCalledWith('seek', 99)
  })

  it('closes the socket on unmount', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    const { unmount } = renderHook(() =>
      useWatchParty('room1', videoRef(makeVideo()), false, '/back'))

    unmount()
    expect(socket.close).toHaveBeenCalled()
  })

  it('defers the sync until loadedmetadata when the video is not ready', () => {
    const socket = makeSocket()
    mockApi.openWatchPartySocket.mockReturnValue(socket)
    const listeners: Record<string, () => void> = {}
    const video = makeVideo({
      readyState: 0,
      currentTime: 0,
      paused: true,
      addEventListener: vi.fn((ev: string, cb: () => void) => { listeners[ev] = cb }),
      removeEventListener: vi.fn(),
    })
    renderHook(() => useWatchParty('room1', videoRef(video), false, '/back'))

    act(() => { socket.emit({ type: 'state', position: 40, playing: false }) })
    // Not applied yet — registered a loadedmetadata handler instead.
    expect(video.currentTime).toBe(0)
    expect(listeners.loadedmetadata).toBeTypeOf('function')

    act(() => { listeners.loadedmetadata() })
    expect(video.currentTime).toBe(40)
  })
})
