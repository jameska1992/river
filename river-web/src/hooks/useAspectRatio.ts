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

// useAspectRatio persists user fit/zoom preferences across reloads so that
// e.g. someone watching cropped 4:3 content can set "Stretch" once and have
// it stick. State is shared via localStorage — opening a second tab will
// pick up the same defaults but won't live-sync (no storage listener
// because we'd then have to deal with intra-session race conditions).
export function useAspectRatio() {
  const [state, setState] = useState<AspectRatioState>(load)

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
    } catch {
      // localStorage can be disabled or full; non-fatal.
    }
  }, [state])

  const setFitMode = useCallback((m: FitMode) => {
    setState(s => ({ ...s, fitMode: m }))
  }, [])

  const setZoom = useCallback((z: number) => {
    setState(s => ({ ...s, zoom: clamp(z) }))
  }, [])

  const zoomIn  = useCallback(() => setState(s => ({ ...s, zoom: clamp(s.zoom + ZOOM_STEP) })), [])
  const zoomOut = useCallback(() => setState(s => ({ ...s, zoom: clamp(s.zoom - ZOOM_STEP) })), [])
  const reset   = useCallback(() => setState(DEFAULT_STATE), [])

  return { ...state, setFitMode, setZoom, zoomIn, zoomOut, reset }
}
