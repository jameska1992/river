import { useEffect, useState } from 'react'
import type { PlaybackEngine } from '../core/PlaybackEngine'
import { formatTime } from '../core/time'
import type { EngineDiagnostics, MediaTrackInfo } from '../core/types'
import { channelLabel } from '../mediabunny/MediaTrackAdapter'
import { detectWebCodecs, probeRiverCodecs, type RiverCodecSupport } from '../mediabunny/capabilities'
import { usePlaybackState } from './usePlaybackState'
import styles from './DebugPanel.module.css'

function formatBytes(n: number | undefined): string {
  if (n === undefined) return '—'
  if (n < 1024) return `${n} B`
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KiB`
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MiB`
  return `${(n / 1024 ** 3).toFixed(2)} GiB`
}

function describeTrack(t: MediaTrackInfo | null | undefined): string {
  if (!t) return '—'
  const parts = [t.codec?.toUpperCase(), t.codecString]
  if (t.width && t.height) parts.push(`${t.width}×${t.height}`)
  if (t.channels) parts.push(channelLabel(t.channels))
  if (t.sampleRate) parts.push(`${t.sampleRate} Hz`)
  return parts.filter(Boolean).join(' · ')
}

interface Props {
  engine: PlaybackEngine | null
  reason?: string
}

/** Live engine internals for the prototype page. Diagnostics are polled; playback state is subscribed. */
export function DebugPanel({ engine, reason }: Props) {
  const state = usePlaybackState(engine)
  const [diag, setDiag] = useState<EngineDiagnostics>({})
  const [codecs, setCodecs] = useState<RiverCodecSupport | null>(null)
  const webcodecs = detectWebCodecs()

  useEffect(() => { void probeRiverCodecs().then(setCodecs) }, [])

  useEffect(() => {
    if (!engine) return
    const tick = () => setDiag(engine.getDiagnostics())
    tick()
    const id = window.setInterval(tick, 1000)
    return () => window.clearInterval(id)
  }, [engine])

  const activeAudio = state.audioTracks.find(t => t.id === state.activeAudioTrackId)
  const rows: [string, string][] = [
    ['Backend', engine ? `${engine.kind}${reason ? ` (${reason})` : ''}` : '—'],
    ['Status', state.status + (state.seeking ? ' (seeking)' : '')],
    ['Position', formatTime(state.position) + ` (${state.position.toFixed(2)}s)`],
    ['Duration', formatTime(state.duration)],
    ['Buffer ahead', `${state.bufferedAhead.toFixed(1)} s`],
    ['Video', describeTrack(state.videoTrack)],
    ['Audio', describeTrack(activeAudio) + (state.audioTracks.length > 1 ? ` (${state.audioTracks.length} tracks)` : '')],
    ['Volume', state.muted ? 'muted' : `${Math.round(state.volume * 100)}%`],
    ['WebCodecs', webcodecs.missing.length ? `missing ${webcodecs.missing.join(', ')}` : 'available'],
    ['H.264 / AAC decode', codecs ? `${codecs.h264 ? 'yes' : 'NO'} / ${codecs.aac ? 'yes' : 'NO'}` : '…'],
    ['Source', diag.sourceKind ?? '—'],
    ['Range requests', diag.requestCount !== undefined ? String(diag.requestCount) : '—'],
    ['Bytes fetched', diag.fileSize ? `${formatBytes(diag.bytesFetched)} of ${formatBytes(diag.fileSize)} (${(((diag.bytesFetched ?? 0) / diag.fileSize) * 100).toFixed(1)}%)` : formatBytes(diag.bytesFetched)],
    ['Dropped frames', diag.droppedFrames !== undefined ? String(diag.droppedFrames) : '—'],
    ['A/V offset', diag.avSync ? `mean ${diag.avSync.meanMs.toFixed(1)} ms · max ${diag.avSync.maxMs.toFixed(1)} ms (${diag.avSync.samples} frames)` : '—'],
    ['Audio latency comp.', diag.audioOutputLatencyMs !== undefined ? `${diag.audioOutputLatencyMs.toFixed(1)} ms` : '—'],
    ['Late audio starts', diag.lateAudioStarts !== undefined ? String(diag.lateAudioStarts) : '—'],
  ]

  return (
    <div className={styles.panel}>
      <dl className={styles.grid}>
        {rows.map(([k, v]) => (
          <div key={k} className={styles.row}><dt>{k}</dt><dd>{v}</dd></div>
        ))}
      </dl>
      {state.error && <p className={styles.error}>Error ({state.error.code}{state.error.recoverable ? ', recoverable' : ''}): {state.error.message}</p>}
      {diag.notes && diag.notes.length > 0 && (
        <ul className={styles.notes}>{diag.notes.map((n, i) => <li key={i}>{n}</li>)}</ul>
      )}
    </div>
  )
}
