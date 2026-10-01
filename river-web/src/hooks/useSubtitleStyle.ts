import { useCallback, useEffect, useState } from 'react'

export interface SubtitleStyleState {
  fontScale: number  // multiplier on the player's base caption size; clamped to [MIN_SCALE, MAX_SCALE]
  color: string      // caption text colour; one of SUBTITLE_COLORS
  bgOpacity: number  // 0..1 opacity of the black box behind the text
}

const STORAGE_KEY = 'river:subtitleStyle'

export const MIN_SCALE = 0.5
export const MAX_SCALE = 3.0
export const SCALE_STEP = 0.25

// Named font-size presets the menu exposes as buttons. Persistence stores the
// raw scale factor (not the preset name) so a value set via +/- still survives.
export const FONT_PRESETS: { label: string; scale: number }[] = [
  { label: 'Small', scale: 0.75 },
  { label: 'Medium', scale: 1 },
  { label: 'Large', scale: 1.5 },
  { label: 'XL', scale: 2 },
]

// Small high-contrast palette. White is the default; yellow is the classic
// broadcast-caption colour that reads over most scenes.
export const SUBTITLE_COLORS: { label: string; value: string }[] = [
  { label: 'White', value: '#ffffff' },
  { label: 'Yellow', value: '#ffeb3b' },
  { label: 'Cyan', value: '#4dd0e1' },
  { label: 'Green', value: '#81c784' },
]

export const DEFAULT_STATE: SubtitleStyleState = {
  fontScale: 1,
  color: '#ffffff',
  bgOpacity: 0.75,
}

const COLOR_VALUES = SUBTITLE_COLORS.map(c => c.value)

function clampScale(s: number) {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, Math.round(s * 100) / 100))
}

function clampOpacity(o: number) {
  return Math.min(1, Math.max(0, Math.round(o * 100) / 100))
}

function load(): SubtitleStyleState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return DEFAULT_STATE
    const parsed = JSON.parse(raw) as Partial<SubtitleStyleState>
    return {
      fontScale: clampScale(typeof parsed.fontScale === 'number' ? parsed.fontScale : 1),
      color: typeof parsed.color === 'string' && COLOR_VALUES.includes(parsed.color)
        ? parsed.color
        : DEFAULT_STATE.color,
      bgOpacity: clampOpacity(typeof parsed.bgOpacity === 'number' ? parsed.bgOpacity : DEFAULT_STATE.bgOpacity),
    }
  } catch {
    return DEFAULT_STATE
  }
}

// useSubtitleStyle persists caption appearance (size, colour, background
// opacity) across reloads and across titles, mirroring useAspectRatio. State
// is shared via localStorage under one key for the whole app — pick a size
// once and every subsequent watch page honours it. Same key as river-tv so a
// browser that runs both frontends shares caption preferences. No cross-tab
// live sync (no storage event listener, to avoid intra-session races).
export function useSubtitleStyle() {
  const [state, setState] = useState<SubtitleStyleState>(load)

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
    } catch {
      // localStorage can be disabled or full; non-fatal.
    }
  }, [state])

  const setFontScale = useCallback((s: number) => {
    setState(st => ({ ...st, fontScale: clampScale(s) }))
  }, [])

  const setColor = useCallback((c: string) => {
    setState(st => ({ ...st, color: COLOR_VALUES.includes(c) ? c : st.color }))
  }, [])

  const setBgOpacity = useCallback((o: number) => {
    setState(st => ({ ...st, bgOpacity: clampOpacity(o) }))
  }, [])

  const scaleUp = useCallback(() => setState(st => ({ ...st, fontScale: clampScale(st.fontScale + SCALE_STEP) })), [])
  const scaleDown = useCallback(() => setState(st => ({ ...st, fontScale: clampScale(st.fontScale - SCALE_STEP) })), [])
  const reset = useCallback(() => setState(DEFAULT_STATE), [])

  return { ...state, setFontScale, setColor, setBgOpacity, scaleUp, scaleDown, reset }
}
