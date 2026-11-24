// WebVTT parsing for the subtitle overlay. A copy of the parser private to
// MovieWatchPage (the production page is intentionally left untouched during
// the prototype); the two should be merged when that page migrates onto the
// player subsystem.

export interface VTTCue { start: number; end: number; text: string }

export function parseVTTTime(s: string): number {
  const parts = s.trim().split(':')
  if (parts.length === 3) return +parts[0] * 3600 + +parts[1] * 60 + parseFloat(parts[2])
  return +parts[0] * 60 + parseFloat(parts[1])
}

export function parseVTT(content: string): VTTCue[] {
  const cues: VTTCue[] = []
  for (const block of content.replace(/\r\n/g, '\n').split(/\n\n+/)) {
    const lines = block.trim().split('\n')
    const ti = lines.findIndex(l => l.includes('-->'))
    if (ti === -1) continue
    const [start, end] = lines[ti].split('-->').map(s => parseVTTTime(s.trim().split(/\s+/)[0]))
    const text = lines.slice(ti + 1).join('\n').replace(/<[^>]+>/g, '').trim()
    // Skip empty cues and ASS drawing/animation cues (path coordinate data)
    if (!text || !/[a-zA-Z]{2}|[À-￿]/.test(text)) continue
    cues.push({ start, end, text })
  }
  return cues
}

/** The cue showing at `time` (offset applied: positive offset delays subtitles), or ''. */
export function activeCueText(cues: VTTCue[], time: number, offset = 0): string {
  const t = time - offset
  return cues.find(c => t >= c.start && t <= c.end)?.text ?? ''
}
