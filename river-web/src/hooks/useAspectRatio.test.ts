import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import {
  useAspectRatio,
  MIN_ZOOM, MAX_ZOOM, ZOOM_STEP,
} from './useAspectRatio'

const STORAGE_KEY = 'river:aspectRatio'

beforeEach(() => {
  localStorage.clear()
})

describe('useAspectRatio', () => {
  it('defaults to contain / zoom 1 with no stored state', () => {
    const { result } = renderHook(() => useAspectRatio())
    expect(result.current.fitMode).toBe('contain')
    expect(result.current.zoom).toBe(1)
  })

  it('loads a previously stored state', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ fitMode: 'cover', zoom: 1.5 }))
    const { result } = renderHook(() => useAspectRatio())
    expect(result.current.fitMode).toBe('cover')
    expect(result.current.zoom).toBe(1.5)
  })

  it('falls back to defaults on invalid JSON', () => {
    localStorage.setItem(STORAGE_KEY, '{not json')
    const { result } = renderHook(() => useAspectRatio())
    expect(result.current.fitMode).toBe('contain')
    expect(result.current.zoom).toBe(1)
  })

  it('sanitises an unknown fitMode and clamps an out-of-range stored zoom', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ fitMode: 'bogus', zoom: 99 }))
    const { result } = renderHook(() => useAspectRatio())
    expect(result.current.fitMode).toBe('contain')
    expect(result.current.zoom).toBe(MAX_ZOOM)
  })

  it('setFitMode updates the mode and persists it', () => {
    const { result } = renderHook(() => useAspectRatio())
    act(() => result.current.setFitMode('fill'))
    expect(result.current.fitMode).toBe('fill')
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY)!).fitMode).toBe('fill')
  })

  it('setZoom clamps below MIN_ZOOM and above MAX_ZOOM', () => {
    const { result } = renderHook(() => useAspectRatio())
    act(() => result.current.setZoom(10))
    expect(result.current.zoom).toBe(MAX_ZOOM)
    act(() => result.current.setZoom(0))
    expect(result.current.zoom).toBe(MIN_ZOOM)
  })

  it('zoomIn / zoomOut step by ZOOM_STEP and clamp at the bounds', () => {
    const { result } = renderHook(() => useAspectRatio())
    act(() => result.current.zoomIn())
    expect(result.current.zoom).toBeCloseTo(1 + ZOOM_STEP, 5)
    act(() => result.current.zoomOut())
    expect(result.current.zoom).toBeCloseTo(1, 5)

    // Walk past MAX_ZOOM and confirm it clamps.
    act(() => result.current.setZoom(MAX_ZOOM))
    act(() => result.current.zoomIn())
    expect(result.current.zoom).toBe(MAX_ZOOM)
  })

  it('reset restores the defaults', () => {
    const { result } = renderHook(() => useAspectRatio())
    act(() => { result.current.setFitMode('cover'); result.current.setZoom(1.8) })
    act(() => result.current.reset())
    expect(result.current.fitMode).toBe('contain')
    expect(result.current.zoom).toBe(1)
  })

  it('persists the current state to localStorage on change', () => {
    const { result } = renderHook(() => useAspectRatio())
    act(() => result.current.setZoom(1.25))
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!)
    expect(stored).toEqual({ fitMode: 'contain', zoom: 1.25 })
  })
})
