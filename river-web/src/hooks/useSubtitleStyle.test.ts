import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import {
  useSubtitleStyle,
  DEFAULT_STATE,
  MIN_SCALE, MAX_SCALE, SCALE_STEP,
} from './useSubtitleStyle'

const STORAGE_KEY = 'river:subtitleStyle'

beforeEach(() => {
  localStorage.clear()
})

describe('useSubtitleStyle', () => {
  it('defaults to white / scale 1 / 0.75 opacity with no stored state', () => {
    const { result } = renderHook(() => useSubtitleStyle())
    expect(result.current.fontScale).toBe(1)
    expect(result.current.color).toBe('#ffffff')
    expect(result.current.bgOpacity).toBe(0.75)
  })

  it('loads a previously stored state', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ fontScale: 1.5, color: '#ffeb3b', bgOpacity: 0.5 }))
    const { result } = renderHook(() => useSubtitleStyle())
    expect(result.current.fontScale).toBe(1.5)
    expect(result.current.color).toBe('#ffeb3b')
    expect(result.current.bgOpacity).toBe(0.5)
  })

  it('falls back to defaults on invalid JSON', () => {
    localStorage.setItem(STORAGE_KEY, '{not json')
    const { result } = renderHook(() => useSubtitleStyle())
    expect(result.current).toMatchObject(DEFAULT_STATE)
  })

  it('clamps an out-of-range stored scale and rejects an unknown colour', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ fontScale: 99, color: 'rebeccapurple', bgOpacity: 5 }))
    const { result } = renderHook(() => useSubtitleStyle())
    expect(result.current.fontScale).toBe(MAX_SCALE)
    expect(result.current.color).toBe(DEFAULT_STATE.color)
    expect(result.current.bgOpacity).toBe(1)
  })

  it('persists updates to localStorage', () => {
    const { result } = renderHook(() => useSubtitleStyle())
    act(() => result.current.setFontScale(1.5))
    act(() => result.current.setColor('#4dd0e1'))
    act(() => result.current.setBgOpacity(0.25))
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!)
    expect(stored).toEqual({ fontScale: 1.5, color: '#4dd0e1', bgOpacity: 0.25 })
  })

  it('ignores a colour outside the palette', () => {
    const { result } = renderHook(() => useSubtitleStyle())
    act(() => result.current.setColor('#123456'))
    expect(result.current.color).toBe('#ffffff')
  })

  it('steps and clamps the scale with scaleUp / scaleDown', () => {
    const { result } = renderHook(() => useSubtitleStyle())
    act(() => result.current.setFontScale(MAX_SCALE))
    act(() => result.current.scaleUp())
    expect(result.current.fontScale).toBe(MAX_SCALE)

    act(() => result.current.setFontScale(MIN_SCALE))
    act(() => result.current.scaleDown())
    expect(result.current.fontScale).toBe(MIN_SCALE)

    act(() => result.current.setFontScale(1))
    act(() => result.current.scaleUp())
    expect(result.current.fontScale).toBe(1 + SCALE_STEP)
  })

  it('reset returns to defaults', () => {
    const { result } = renderHook(() => useSubtitleStyle())
    act(() => result.current.setFontScale(2))
    act(() => result.current.setColor('#ffeb3b'))
    act(() => result.current.reset())
    expect(result.current).toMatchObject(DEFAULT_STATE)
  })
})
