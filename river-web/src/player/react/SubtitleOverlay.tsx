import { useEffect, useState } from 'react'
import { activeCueText, parseVTT, type VTTCue } from '../subtitles/vtt'
import styles from './Player.module.css'

interface Props {
  /** Authenticated WebVTT URL, or null for off. */
  url: string | null
  position: number
  offset?: number
}

/**
 * River's subtitle renderer as an independent layer over any engine's
 * surface: fetches + parses the sidecar VTT and shows the cue for `position`.
 * Nothing is burned into video frames, so it's identical for canvas and
 * <video> output.
 */
export function SubtitleOverlay({ url, position, offset = 0 }: Props) {
  const [loaded, setLoaded] = useState<{ url: string; cues: VTTCue[] } | null>(null)

  useEffect(() => {
    if (!url) return
    const ctrl = new AbortController()
    fetch(url, { signal: ctrl.signal })
      .then(r => r.text())
      .then(text => setLoaded({ url, cues: parseVTT(text) }))
      .catch(() => { if (!ctrl.signal.aborted) setLoaded({ url, cues: [] }) })
    return () => ctrl.abort()
  }, [url])

  const cues = url && loaded?.url === url ? loaded.cues : []
  const text = activeCueText(cues, position, offset)
  if (!text) return null
  return <div className={styles.subtitle}>{text}</div>
}
