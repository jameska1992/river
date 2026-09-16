// Pure playback helpers, kept separate from the <video> element so they can
// be unit-tested without a real HTMLMediaElement (jsdom has none).

// How far the ±skip buttons jump.
export const SEEK_SECONDS = 10
// Below this playhead, Skip-back jumps to the previous item (if any);
// past it, Skip-back just restarts the current item.
export const SKIP_PREV_THRESHOLD_S = 10
// Show the "Up Next" card during the final stretch of the current item.
export const UP_NEXT_THRESHOLD_S = 30

const RESUME_MIN_POSITION_S = 5
const RESUME_TAIL_GUARD_S = 30

// Whether a saved position is worth resuming from: meaningfully into the file
// and not within the last 30s (where dropping the user back in feels like a bug).
export function shouldResume(position: number, duration: number): boolean {
  return (
    position > RESUME_MIN_POSITION_S &&
    duration > 0 &&
    position < duration - RESUME_TAIL_GUARD_S
  )
}

// Clamp a relative seek to [0, duration].
export function clampSeek(current: number, deltaS: number, duration: number): number {
  return Math.max(0, Math.min(duration || 0, current + deltaS))
}

// Fraction (0–1) along a bar for a pointer at clientX, given the bar's left
// edge and width. Clamped so drags past either end saturate rather than
// overshoot.
export function seekFraction(clientX: number, rectLeft: number, rectWidth: number): number {
  if (rectWidth <= 0) return 0
  return Math.max(0, Math.min(1, (clientX - rectLeft) / rectWidth))
}

// --- Vertical-swipe gestures (volume / brightness) ---

export type PlayerGesture = 'volume' | 'brightness'

// Which control a swipe drives, by where it starts: left half = volume,
// right half = brightness.
export function gestureSide(startX: number, width: number): PlayerGesture {
  return startX < width / 2 ? 'volume' : 'brightness'
}

// New 0–1 value after dragging `dyUp` pixels upward (positive = up) from
// `startValue`, where a `span`-pixel drag covers the whole 0–1 range. Clamped.
export function gestureValue(startValue: number, dyUp: number, span: number): number {
  if (span <= 0) return startValue
  return Math.max(0, Math.min(1, startValue + dyUp / span))
}

// Whether a move is a deliberate vertical drag — past the dead-zone and more
// vertical than horizontal — rather than a tap or a horizontal swipe.
export function isVerticalDrag(dx: number, dy: number, deadZone = 8): boolean {
  return Math.abs(dy) > deadZone && Math.abs(dy) > Math.abs(dx)
}
