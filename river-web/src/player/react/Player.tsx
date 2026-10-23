import { useCallback, useEffect, useRef, useState } from 'react'
import {
  RiPlayFill, RiPauseFill, RiVolumeUpLine, RiVolumeMuteLine,
  RiFullscreenLine, RiFullscreenExitLine, RiReplay10Fill, RiForward10Fill,
} from 'react-icons/ri'
import type { PlaybackEngine } from '../core/PlaybackEngine'
import { PlaybackRecovery } from '../core/PlaybackRecovery'
import { engineOrder, loadWithFallback, type EnginePreference } from '../core/selectEngine'
import { formatTime } from '../core/time'
import type { EngineKind, PlaybackSource } from '../core/types'
import { MediabunnyEngine } from '../mediabunny/MediabunnyEngine'
import { detectWebCodecs } from '../mediabunny/capabilities'
import { NativeEngine } from '../native/NativeEngine'
import { SubtitleOverlay } from './SubtitleOverlay'
import { usePlaybackState } from './usePlaybackState'
import styles from './Player.module.css'

const SKIP_SECONDS = 10

export interface PlayerEngineInfo {
  engine: PlaybackEngine
  reason: string
}

interface Props {
  /** Memoise this — a new object identity reloads the player. */
  source: PlaybackSource | null
  preference?: EnginePreference
  startAt?: number
  autoplay?: boolean
  subtitleUrl?: string | null
  onEngine?: (info: PlayerEngineInfo | null) => void
  onRecover?: (reason: string) => void
}

function createEngine(kind: EngineKind): PlaybackEngine {
  return kind === 'mediabunny' ? new MediabunnyEngine() : new NativeEngine()
}

function useFullscreen(ref: React.RefObject<HTMLElement | null>) {
  const [isFullscreen, setIsFullscreen] = useState(false)
  useEffect(() => {
    const onChange = () => setIsFullscreen(document.fullscreenElement === ref.current)
    document.addEventListener('fullscreenchange', onChange)
    return () => document.removeEventListener('fullscreenchange', onChange)
  }, [ref])
  const toggle = useCallback(() => {
    if (document.fullscreenElement) void document.exitFullscreen()
    else void ref.current?.requestFullscreen()
  }, [ref])
  return { isFullscreen, toggle }
}

/**
 * Prototype River player: owns the controls and overlays, delegates all media
 * work to a PlaybackEngine chosen by capability.
 */
export function Player({ source, preference = 'auto', startAt, autoplay, subtitleUrl = null, onEngine, onRecover }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const surfaceRef = useRef<HTMLDivElement>(null)
  const [engine, setEngine] = useState<PlaybackEngine | null>(null)
  const state = usePlaybackState(engine)
  const { isFullscreen, toggle: toggleFullscreen } = useFullscreen(containerRef)
  const [scrub, setScrub] = useState<number | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  // Callbacks are read through refs so a parent re-render doesn't reload media.
  const onEngineRef = useRef(onEngine)
  const onRecoverRef = useRef(onRecover)
  useEffect(() => { onEngineRef.current = onEngine; onRecoverRef.current = onRecover })

  useEffect(() => {
    const surface = surfaceRef.current
    if (!source || !surface) return
    let cancelled = false
    let current: PlaybackEngine | null = null
    let recovery: PlaybackRecovery | null = null

    const order = engineOrder(preference, detectWebCodecs().videoDecoder)
    void loadWithFallback(order, createEngine, surface, source, { startAt, autoplay }, e => {
      if (cancelled) { e.destroy(); return }
      current = e
      setEngine(e)
    }).then(({ engine: e, reason }) => {
      if (cancelled) return
      recovery = new PlaybackRecovery(e, { onRecover: r => onRecoverRef.current?.(r) })
      onEngineRef.current?.({ engine: e, reason })
    }).catch(() => {})

    return () => {
      cancelled = true
      recovery?.dispose()
      current?.destroy()
      setEngine(null)
      onEngineRef.current?.(null)
    }
    // startAt/autoplay only apply to the initial load of a source.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, preference])

  const run = useCallback((p: Promise<unknown> | void) => {
    if (!p) return
    p.then(() => setActionError(null)).catch((err: unknown) => setActionError(err instanceof Error ? err.message : String(err)))
  }, [])

  const togglePlay = useCallback(() => {
    if (!engine) return
    const st = engine.getState().status
    if (st === 'playing' || st === 'buffering') engine.pause()
    else run(engine.play())
  }, [engine, run])

  const skip = useCallback((d: number) => {
    if (engine) run(engine.seek(engine.getState().position + d))
  }, [engine, run])

  const toggleMute = useCallback(() => {
    if (engine) engine.setMuted(!engine.getState().muted)
  }, [engine])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLSelectElement) return
      switch (e.key) {
        case ' ': case 'k': e.preventDefault(); togglePlay(); break
        case 'ArrowLeft': e.preventDefault(); skip(-SKIP_SECONDS); break
        case 'ArrowRight': e.preventDefault(); skip(SKIP_SECONDS); break
        case 'm': toggleMute(); break
        case 'f': toggleFullscreen(); break
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [togglePlay, skip, toggleMute, toggleFullscreen])

  const commitScrub = () => {
    if (engine && scrub !== null) run(engine.seek(scrub))
    setScrub(null)
  }

  const shownPosition = scrub ?? state.position
  const progress = state.duration > 0 ? (shownPosition / state.duration) * 100 : 0
  const bufferedEnd = state.duration > 0 ? ((state.position + state.bufferedAhead) / state.duration) * 100 : 0
  const isPlaying = state.status === 'playing' || state.status === 'buffering'
  const busy = state.status === 'loading' || state.status === 'buffering' || state.seeking
  const error = state.error?.message ?? actionError

  return (
    <div ref={containerRef} className={styles.player} data-status={state.status}>
      <div ref={surfaceRef} className={styles.surface} onClick={togglePlay} onDoubleClick={toggleFullscreen} />

      <SubtitleOverlay url={subtitleUrl} position={state.position} />

      {busy && <div className={styles.spinner} role="status" aria-label={state.status === 'loading' ? 'Loading' : 'Buffering'} />}

      {error && (
        <div className={styles.error} role="alert">
          <p>{error}</p>
          {engine && state.status === 'error' && (
            <button className="btn btn-secondary" onClick={() => run(engine.reload())}>Retry</button>
          )}
        </div>
      )}

      <div className={styles.controls}>
        <div className={styles.seekWrap}>
          <div className={styles.bufferBar} style={{ width: `${Math.min(100, bufferedEnd)}%` }} />
          <input
            type="range" aria-label="Seek" className={styles.seek}
            min={0} max={state.duration || 0} step={0.1} value={shownPosition}
            disabled={!engine || state.duration <= 0}
            style={{ '--progress': `${progress}%` } as React.CSSProperties}
            onChange={e => setScrub(parseFloat(e.target.value))}
            onPointerUp={commitScrub}
            onKeyUp={commitScrub}
          />
        </div>
        <div className={styles.row}>
          <button className="btn btn-icon" onClick={() => skip(-SKIP_SECONDS)} aria-label={`Rewind ${SKIP_SECONDS} seconds`} disabled={!engine}>
            <RiReplay10Fill size={20} />
          </button>
          <button className="btn btn-icon" onClick={togglePlay} aria-label={isPlaying ? 'Pause' : 'Play'} disabled={!engine}>
            {isPlaying ? <RiPauseFill size={22} /> : <RiPlayFill size={22} />}
          </button>
          <button className="btn btn-icon" onClick={() => skip(SKIP_SECONDS)} aria-label={`Forward ${SKIP_SECONDS} seconds`} disabled={!engine}>
            <RiForward10Fill size={20} />
          </button>
          <button className="btn btn-icon" onClick={toggleMute} aria-label={state.muted ? 'Unmute' : 'Mute'} disabled={!engine}>
            {state.muted || state.volume === 0 ? <RiVolumeMuteLine size={20} /> : <RiVolumeUpLine size={20} />}
          </button>
          <input
            type="range" aria-label="Volume" className={styles.volume}
            min={0} max={1} step={0.02} value={state.muted ? 0 : state.volume}
            onChange={e => {
              const v = parseFloat(e.target.value)
              engine?.setVolume(v)
              engine?.setMuted(v === 0)
            }}
          />
          <span className={styles.time}>{formatTime(shownPosition)} / {formatTime(state.duration)}</span>
          <span className={styles.spacer} />
          {engine?.capabilities.audioTrackSwitching && state.audioTracks.length > 1 && (
            <select
              aria-label="Audio track" className={styles.select}
              value={state.activeAudioTrackId ?? ''}
              onChange={e => run(engine.setAudioTrack(e.target.value))}
            >
              {state.audioTracks.map(t => <option key={t.id} value={t.id}>{t.label}</option>)}
            </select>
          )}
          <button className="btn btn-icon" onClick={toggleFullscreen} aria-label={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}>
            {isFullscreen ? <RiFullscreenExitLine size={20} /> : <RiFullscreenLine size={20} />}
          </button>
        </div>
      </div>
    </div>
  )
}
