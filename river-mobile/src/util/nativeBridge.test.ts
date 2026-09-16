import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  hasNativeAudio, nativeAudioActive, nativeSetMetadata, nativeSetPlayback, registerNativeControls,
} from './nativeBridge'

describe('nativeBridge (no host present)', () => {
  beforeEach(() => { delete window.RiverNative })

  it('reports no native audio and no-ops without a host', () => {
    expect(hasNativeAudio()).toBe(false)
    // None of these should throw when RiverNative is absent.
    expect(() => nativeAudioActive(true)).not.toThrow()
    expect(() => nativeSetMetadata('t', 'a', 'al', 1000)).not.toThrow()
    expect(() => nativeSetPlayback(true, 500)).not.toThrow()
  })
})

describe('nativeBridge (host present)', () => {
  const host = {
    audioActive: vi.fn(),
    setMetadata: vi.fn(),
    setPlayback: vi.fn(),
  }
  beforeEach(() => { window.RiverNative = host; vi.clearAllMocks() })

  it('forwards calls to the host', () => {
    expect(hasNativeAudio()).toBe(true)
    nativeAudioActive(true)
    expect(host.audioActive).toHaveBeenCalledWith(true)
    nativeSetMetadata('So What', 'Miles Davis', 'Kind of Blue', 545_000)
    expect(host.setMetadata).toHaveBeenCalledWith('So What', 'Miles Davis', 'Kind of Blue', 545000)
    nativeSetPlayback(true, 1234.6)
    expect(host.setPlayback).toHaveBeenCalledWith(true, 1235) // rounded
  })

  it('clamps/normalises non-finite numbers to 0', () => {
    nativeSetMetadata('t', 'a', 'al', Number.NaN)
    expect(host.setMetadata).toHaveBeenCalledWith('t', 'a', 'al', 0)
    nativeSetPlayback(false, -5)
    expect(host.setPlayback).toHaveBeenCalledWith(false, 0)
  })
})

describe('registerNativeControls', () => {
  it('exposes controls on window and cleans them up', () => {
    const controls = { play: vi.fn(), pause: vi.fn(), next: vi.fn(), prev: vi.fn(), seekTo: vi.fn() }
    const cleanup = registerNativeControls(controls)
    expect(window.__riverAudio).toBe(controls)
    window.__riverAudio!.seekTo(4000)
    expect(controls.seekTo).toHaveBeenCalledWith(4000)
    cleanup()
    expect(window.__riverAudio).toBeUndefined()
  })
})
