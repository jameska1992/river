import { useCallback, useEffect, useState } from 'react'

export type FitMode = 'contain' | 'fill' | 'cover'

export interface AspectRatioState {
  fitMode: FitMode
  zoom: number  // 1.0 = 100%; clamped to [MIN_ZOOM, MAX_ZOOM]
}

const STORAGE_KEY = 'river:aspectRatio'

export const MIN_ZOOM = 0.5
export const MAX_ZOOM = 2.0
export const ZOOM_STEP = 0.05

const DEFAULT_STATE: AspectRatioState = { fitMode: 'contain', zoom: 1 }

function clamp(z: number) {
  return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round(z * 100) / 100))
}

function load(): AspectRatioState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return DEFAULT_STATE
    const parsed = JSON.parse(raw) as Partial<AspectRatioState>
    return {
      fitMode: parsed.fitMode === 'fill' || parsed.fitMode === 'cover' ? parsed.fitMode : 'contain',
      zoom: clamp(typeof parsed.zoom === 'number' ? parsed.zoom : 1),
    }
  } catch {
    return DEFAULT_STATE
  }
}

// Persists fit/zoom preferences across reloads — same storage key as
// river-web so the same browser shares preferences across both
// frontends. No cross-tab live sync (no storage event listener).
export function useAspectRatio() {
  const [state, setState] = useState<AspectRatioState>(load)

  useEffect(() => {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)) }
    catch { /* disabled or full — non-fatal */ }
  }, [state])

  const setFitMode = useCallback((m: FitMode) => setState(s => ({ ...s, fitMode: m })), [])
  const zoomIn = useCallback(() => setState(s => ({ ...s, zoom: clamp(s.zoom + ZOOM_STEP) })), [])
  const zoomOut = useCallback(() => setState(s => ({ ...s, zoom: clamp(s.zoom - ZOOM_STEP) })), [])
  const reset = useCallback(() => setState(DEFAULT_STATE), [])

  return { ...state, setFitMode, zoomIn, zoomOut, reset }
}
