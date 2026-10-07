import type { TimeRange } from './types'

/** h:mm:ss / m:ss, matching the existing watch pages. */
export function formatTime(s: number): string {
  if (!Number.isFinite(s) || s < 0) return '0:00'
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = Math.floor(s % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
  return `${m}:${String(sec).padStart(2, '0')}`
}

export function clamp(v: number, min: number, max: number): number {
  return Math.min(Math.max(v, min), max)
}

/** Clamps a seek target into [0, duration]; a non-finite target becomes 0. */
export function clampSeek(target: number, duration: number): number {
  if (!Number.isFinite(target)) return 0
  return duration > 0 ? clamp(target, 0, duration) : Math.max(0, target)
}

/** Seconds of contiguous buffered media from `position` onward. */
export function bufferedAhead(ranges: TimeRange[], position: number, tolerance = 0.1): number {
  for (const r of ranges) {
    if (position >= r.start - tolerance && position <= r.end) return Math.max(0, r.end - position)
  }
  return 0
}

/** Converts a TimeRanges DOM object into plain ranges. */
export function fromTimeRanges(tr: TimeRanges | null | undefined): TimeRange[] {
  if (!tr) return []
  const out: TimeRange[] = []
  for (let i = 0; i < tr.length; i++) out.push({ start: tr.start(i), end: tr.end(i) })
  return out
}
