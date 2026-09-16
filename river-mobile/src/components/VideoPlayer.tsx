import { useCallback, useEffect, useRef, useState } from 'react'
import {
  RiArrowLeftLine, RiPlayFill, RiPauseFill,
  RiReplay10Line, RiForward10Line,
  RiSkipBackFill, RiSkipForwardFill,
  RiClosedCaptioningLine, RiVolumeUpLine,
  RiFullscreenLine, RiFullscreenExitLine,
} from 'react-icons/ri'
import { api, type AudioTrack, type Subtitle } from '../api'
import { imageUrl } from '../util/imageUrl'
import { formatDuration } from '../util/format'
import {
  SEEK_SECONDS, SKIP_PREV_THRESHOLD_S, UP_NEXT_THRESHOLD_S,
  shouldResume, clampSeek, seekFraction,
} from '../util/player'
import { BottomSheet, SheetOption } from './BottomSheet'

const CONTROLS_HIDE_MS = 3500
const PROGRESS_SEND_INTERVAL_S = 5

export interface UpNext {
  title: string
  subtitle?: string
  posterUrl?: string
  onPlay: () => void
}

interface Props {
  streamUrl: string
  // Rebuilds the stream URL with a fresh stream token — used by recovery
  // after a long pause / device sleep. Falls back to streamUrl when omitted.
  buildStreamUrl?: () => string
  title: string
  subtitle?: string
  progressKind: 'movie' | 'episode'
  progressId: string
  fetchSubtitles?: () => Promise<Subtitle[]>
  fetchAudioTracks?: () => Promise<AudioTrack[]>
  upNext?: UpNext
  onPrev?: () => void
  onNext?: () => void
  // Skip resume and start at 0 — set by skip-next / up-next navigations so the
  // freshly-chosen episode doesn't drop the viewer back where they last paused.
  startFromBeginning?: boolean
  onExit: () => void
}

/*
 * Touch-first fullscreen video player for movies + episodes.
 *
 * Owns the <video> (autoplay, resume-from-saved-position, ended → next/exit),
 * throttled progress reporting over the WebSocket, tap-to-toggle controls with
 * a 3.5s auto-hide, a draggable scrub bar, ±10s skips, subtitle (<track> mode
 * toggling) and alternate-audio (muted video + synced <audio>) pickers, an
 * "Up Next" card, and best-effort fullscreen + landscape orientation lock.
 */
export function VideoPlayer({
  streamUrl, buildStreamUrl, title, subtitle,
  progressKind, progressId,
  fetchSubtitles, fetchAudioTracks,
  upNext, onPrev, onNext, startFromBeginning, onExit,
}: Props) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const audioRef = useRef<HTMLAudioElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const barRef = useRef<HTMLDivElement>(null)

  // The <video> plays from local state seeded by the prop so recovery can
  // reload the source in place; keep it in sync when the prop changes
  // (skip-next reuses this component with a new URL) via adjust-during-render.
  const [src, setSrc] = useState(streamUrl)
  const [lastStreamUrl, setLastStreamUrl] = useState(streamUrl)
  if (streamUrl !== lastStreamUrl) {
    setLastStreamUrl(streamUrl)
    setSrc(streamUrl)
  }

  const [paused, setPaused] = useState(true)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [bufferedEnd, setBufferedEnd] = useState(0)
  const [buffering, setBuffering] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showControls, setShowControls] = useState(true)
  const [isFullscreen, setIsFullscreen] = useState(false)

  const [subtitles, setSubtitles] = useState<Subtitle[]>([])
  const [audioTracks, setAudioTracks] = useState<AudioTrack[]>([])
  const [activeSubtitleId, setActiveSubtitleId] = useState<string | null>(null)
  const [activeAudioId, setActiveAudioId] = useState<string | null>(null)
  const [sheet, setSheet] = useState<'subs' | 'audio' | null>(null)

  // Scrubbing: while dragging the bar we show the drag fraction instead of the
  // video's currentTime so the thumb tracks the finger without jitter.
  const [scrubFrac, setScrubFrac] = useState<number | null>(null)
  const scrubbing = scrubFrac !== null
  const [upNextDismissed, setUpNextDismissed] = useState(false)

  const altAudio = activeAudioId !== null
  const altAudioUrl = activeAudioId ? api.audioTrackStreamUrl(activeAudioId) : undefined
  const [audioSrc, setAudioSrc] = useState(altAudioUrl)
  const [lastAudioUrl, setLastAudioUrl] = useState(altAudioUrl)
  if (altAudioUrl !== lastAudioUrl) {
    setLastAudioUrl(altAudioUrl)
    setAudioSrc(altAudioUrl)
  }

  const revealControls = useCallback(() => setShowControls(true), [])

  // Auto-hide controls while playing. Setting state inside the timeout (not
  // synchronously in the effect body) keeps this clear of set-state-in-effect.
  useEffect(() => {
    if (!showControls || paused || scrubbing || sheet) return
    const t = window.setTimeout(() => setShowControls(false), CONTROLS_HIDE_MS)
    return () => window.clearTimeout(t)
  }, [showControls, paused, scrubbing, sheet])

  // Resume from saved position unless told to start fresh.
  useEffect(() => {
    if (!progressId || startFromBeginning) return
    let resumed = false
    api.getProgress(progressKind, progressId).then(p => {
      const v = videoRef.current
      if (resumed || !v || !p) return
      const apply = () => {
        if (shouldResume(p.position, p.duration)) v.currentTime = p.position
        resumed = true
      }
      if (v.readyState >= 1) apply()
      else v.addEventListener('loadedmetadata', apply, { once: true })
    }).catch(() => {})
  }, [progressKind, progressId, startFromBeginning])

  // Throttled progress reporting + a final flush on unmount.
  useEffect(() => {
    if (!progressId) return
    const sock = api.openProgressSocket()
    let lastSent = -PROGRESS_SEND_INTERVAL_S
    const interval = window.setInterval(() => {
      const v = videoRef.current
      if (!v || v.paused || !v.duration) return
      if (Math.abs(v.currentTime - lastSent) >= PROGRESS_SEND_INTERVAL_S) {
        sock.send(progressKind, progressId, v.currentTime, v.duration)
        lastSent = v.currentTime
      }
    }, 1000)
    return () => {
      // eslint-disable-next-line react-hooks/exhaustive-deps -- read the live ref at unmount to flush the final position
      const v = videoRef.current
      if (v && v.duration) sock.send(progressKind, progressId, v.currentTime, v.duration)
      window.clearInterval(interval)
      sock.close()
    }
  }, [progressKind, progressId])

  // Load subtitle + audio-track metadata once per source.
  useEffect(() => {
    let alive = true
    if (fetchSubtitles) fetchSubtitles().then(s => { if (alive) setSubtitles(s) }).catch(() => {})
    if (fetchAudioTracks) fetchAudioTracks().then(a => { if (alive) setAudioTracks(a) }).catch(() => {})
    return () => { alive = false }
  }, [fetchSubtitles, fetchAudioTracks])

  // Toggle subtitle TextTrack modes when the active subtitle changes.
  useEffect(() => {
    const v = videoRef.current
    if (!v) return
    const idx = activeSubtitleId ? subtitles.findIndex(s => s.id === activeSubtitleId) : -1
    for (let i = 0; i < v.textTracks.length; i++) {
      v.textTracks[i].mode = i === idx ? 'showing' : 'hidden'
    }
  }, [activeSubtitleId, subtitles])

  // Alt-audio sync: mirror the video's play/pause/seek onto a separate muted
  // <audio> element carrying the selected track.
  useEffect(() => {
    if (!altAudio) return
    const v = videoRef.current
    const a = audioRef.current
    if (!v || !a) return
    const onPlay = () => { void a.play() }
    const onPauseEv = () => { a.pause() }
    const onSeeked = () => { a.currentTime = v.currentTime }
    const onTimeUpdate = () => {
      if (Math.abs(a.currentTime - v.currentTime) > 0.15) a.currentTime = v.currentTime
    }
    a.currentTime = v.currentTime
    if (!v.paused) void a.play()
    v.addEventListener('play', onPlay)
    v.addEventListener('pause', onPauseEv)
    v.addEventListener('seeked', onSeeked)
    v.addEventListener('timeupdate', onTimeUpdate)
    return () => {
      v.removeEventListener('play', onPlay)
      v.removeEventListener('pause', onPauseEv)
      v.removeEventListener('seeked', onSeeked)
      v.removeEventListener('timeupdate', onTimeUpdate)
      a.pause()
    }
  }, [altAudio, activeAudioId])

  // Track document fullscreen state so the button reflects reality if the user
  // exits via a system gesture.
  useEffect(() => {
    const onFs = () => setIsFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onFs)
    return () => document.removeEventListener('fullscreenchange', onFs)
  }, [])

  const togglePlay = useCallback(() => {
    const v = videoRef.current
    if (!v) return
    if (v.paused) void v.play()
    else v.pause()
    revealControls()
  }, [revealControls])

  const seekBy = useCallback((deltaS: number) => {
    const v = videoRef.current
    if (!v) return
    v.currentTime = clampSeek(v.currentTime, deltaS, v.duration || 0)
    revealControls()
  }, [revealControls])

  // Skip-back: near the start → previous item (if any), else restart at 0.
  const skipBack = useCallback(() => {
    const v = videoRef.current
    if (!v) return
    if (v.currentTime < SKIP_PREV_THRESHOLD_S && onPrev) onPrev()
    else v.currentTime = 0
    revealControls()
  }, [onPrev, revealControls])

  const scrubTo = useCallback((clientX: number) => {
    const el = barRef.current
    const v = videoRef.current
    if (!el || !v || !duration) return
    const r = el.getBoundingClientRect()
    const frac = seekFraction(clientX, r.left, r.width)
    setScrubFrac(frac)
    v.currentTime = frac * duration
  }, [duration])

  const toggleFullscreen = useCallback(async () => {
    revealControls()
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen()
      } else {
        await rootRef.current?.requestFullscreen()
        // Best-effort: lock to landscape once fullscreen (allowed only there).
        const orientation = screen.orientation as (ScreenOrientation & { lock?: (o: string) => Promise<void> }) | undefined
        await orientation?.lock?.('landscape').catch(() => {})
      }
    } catch { /* fullscreen / orientation not supported — no-op */ }
  }, [revealControls])

  const displayedTime = scrubbing && scrubFrac != null ? scrubFrac * duration : currentTime
  const remaining = Math.max(0, duration - displayedTime)
  const progressPct = duration > 0 ? (displayedTime / duration) * 100 : 0
  const bufferedPct = duration > 0 ? (bufferedEnd / duration) * 100 : 0
  const showUpNext = !!upNext && !upNextDismissed && duration > 0 && remaining < UP_NEXT_THRESHOLD_S

  return (
    <div ref={rootRef} style={styles.page}>
      <video
        ref={videoRef}
        src={src}
        autoPlay
        playsInline
        muted={altAudio}
        style={styles.video}
        crossOrigin="anonymous"
        onPlay={() => setPaused(false)}
        onPause={() => setPaused(true)}
        onWaiting={() => setBuffering(true)}
        onPlaying={() => { setBuffering(false); setError(null) }}
        onCanPlay={() => { setBuffering(false); setError(null) }}
        onTimeUpdate={e => setCurrentTime(e.currentTarget.currentTime)}
        onDurationChange={e => setDuration(e.currentTarget.duration)}
        onProgress={e => {
          const v = e.currentTarget
          if (v.buffered.length === 0) return
          for (let i = 0; i < v.buffered.length; i++) {
            if (v.currentTime >= v.buffered.start(i) && v.currentTime <= v.buffered.end(i)) {
              setBufferedEnd(v.buffered.end(i))
              return
            }
          }
          setBufferedEnd(v.buffered.end(v.buffered.length - 1))
        }}
        onEnded={() => (onNext ? onNext() : onExit())}
        onError={async e => {
          const v = e.currentTarget
          if (v.error && v.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) {
            setError('Playback failed')
            // Long pause / expired stream token — refresh it and reload in place.
            try {
              await api.refreshStreamToken()
              setSrc(buildStreamUrl ? buildStreamUrl() : streamUrl)
            } catch { /* stays on the error overlay */ }
          }
        }}
      >
        {subtitles.map(sub => (
          <track
            key={sub.id}
            kind="subtitles"
            label={sub.label || sub.language}
            srcLang={sub.language}
            src={api.subtitleStreamUrl(sub.id)}
          />
        ))}
      </video>

      {altAudio && (
        <audio ref={audioRef} src={audioSrc} autoPlay preload="auto" style={{ display: 'none' }} />
      )}

      {/* Tap surface: toggles the controls. Sits below the controls layer. */}
      <button
        aria-label={showControls ? 'Hide controls' : 'Show controls'}
        style={styles.tapLayer}
        onClick={() => setShowControls(v => !v)}
      />

      {buffering && !error && <div style={styles.spinner}>Loading…</div>}
      {error && (
        <div style={styles.error}>
          {error}
          <button className="btn" onClick={onExit} style={{ marginTop: '0.75rem' }}>Back</button>
        </div>
      )}

      {showUpNext && upNext && (
        <button
          style={styles.upNext}
          onClick={() => { setUpNextDismissed(true); upNext.onPlay() }}
        >
          {imageUrl(upNext.posterUrl, 'backdrop') && (
            <img src={imageUrl(upNext.posterUrl, 'backdrop')} alt="" style={styles.upNextThumb} />
          )}
          <span style={styles.upNextInfo}>
            <span style={styles.upNextLabel}>Up Next</span>
            <span style={styles.upNextTitle}>{upNext.title}</span>
            {upNext.subtitle && <span style={styles.upNextSubtitle}>{upNext.subtitle}</span>}
          </span>
        </button>
      )}

      <div style={{ ...styles.controls, opacity: showControls ? 1 : 0, pointerEvents: showControls ? 'auto' : 'none' }}>
        {/* Top bar */}
        <div style={styles.topBar}>
          <button aria-label="Back" onClick={onExit} style={styles.iconBtn}><RiArrowLeftLine /></button>
          <div style={styles.titleWrap}>
            <div style={styles.title}>{title}</div>
            {subtitle && <div style={styles.subtitle}>{subtitle}</div>}
          </div>
          <button aria-label={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'} onClick={toggleFullscreen} style={styles.iconBtn}>
            {isFullscreen ? <RiFullscreenExitLine /> : <RiFullscreenLine />}
          </button>
        </div>

        {/* Center transport */}
        <div style={styles.center}>
          {onPrev && <button aria-label="Previous" onClick={skipBack} style={styles.sideBtn}><RiSkipBackFill /></button>}
          <button aria-label={`Rewind ${SEEK_SECONDS} seconds`} onClick={() => seekBy(-SEEK_SECONDS)} style={styles.sideBtn}><RiReplay10Line /></button>
          <button aria-label={paused ? 'Play' : 'Pause'} onClick={togglePlay} style={styles.playBtn}>
            {paused ? <RiPlayFill /> : <RiPauseFill />}
          </button>
          <button aria-label={`Forward ${SEEK_SECONDS} seconds`} onClick={() => seekBy(SEEK_SECONDS)} style={styles.sideBtn}><RiForward10Line /></button>
          {onNext && <button aria-label="Next" onClick={onNext} style={styles.sideBtn}><RiSkipForwardFill /></button>}
        </div>

        {/* Bottom: scrub + times + track pickers */}
        <div style={styles.bottom}>
          <div style={styles.timeRow}>
            <span style={styles.time}>{formatDuration(Math.floor(displayedTime))}</span>
            <span style={styles.time}>-{formatDuration(Math.ceil(remaining))}</span>
          </div>
          <div
            ref={barRef}
            role="slider"
            aria-label="Seek"
            aria-valuemin={0}
            aria-valuemax={Math.floor(duration)}
            aria-valuenow={Math.floor(displayedTime)}
            style={styles.barHit}
            onPointerDown={e => {
              (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
              scrubTo(e.clientX)
            }}
            onPointerMove={e => { if (scrubbing) scrubTo(e.clientX) }}
            onPointerUp={() => setScrubFrac(null)}
            onPointerCancel={() => setScrubFrac(null)}
          >
            <div style={styles.barTrack}>
              <div style={{ ...styles.barBuffered, width: `${bufferedPct}%` }} />
              <div style={{ ...styles.barFill, width: `${progressPct}%` }} />
              <div style={{ ...styles.barThumb, left: `${progressPct}%` }} />
            </div>
          </div>

          <div style={styles.trackRow}>
            {subtitles.length > 0 && (
              <button
                aria-label="Subtitles"
                onClick={() => setSheet('subs')}
                style={{ ...styles.iconBtn, ...(activeSubtitleId ? styles.iconBtnActive : {}) }}
              >
                <RiClosedCaptioningLine />
              </button>
            )}
            {audioTracks.length > 0 && (
              <button
                aria-label="Audio track"
                onClick={() => setSheet('audio')}
                style={{ ...styles.iconBtn, ...(activeAudioId ? styles.iconBtnActive : {}) }}
              >
                <RiVolumeUpLine />
              </button>
            )}
          </div>
        </div>
      </div>

      {sheet === 'subs' && (
        <BottomSheet title="Subtitles" onClose={() => setSheet(null)}>
          <SheetOption label="Off" active={activeSubtitleId === null} onSelect={() => { setActiveSubtitleId(null); setSheet(null) }} />
          {subtitles.map(s => (
            <SheetOption
              key={s.id}
              label={s.label || s.language}
              active={activeSubtitleId === s.id}
              onSelect={() => { setActiveSubtitleId(s.id); setSheet(null) }}
            />
          ))}
        </BottomSheet>
      )}

      {sheet === 'audio' && (
        <BottomSheet title="Audio" onClose={() => setSheet(null)}>
          <SheetOption label="Default (in-file)" active={activeAudioId === null} onSelect={() => { setActiveAudioId(null); setSheet(null) }} />
          {audioTracks.map(t => (
            <SheetOption
              key={t.id}
              label={t.label || t.language || `Track ${t.stream_index}`}
              active={activeAudioId === t.id}
              onSelect={() => { setActiveAudioId(t.id); setSheet(null) }}
            />
          ))}
        </BottomSheet>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  page: { position: 'fixed', inset: 0, background: '#000', overflow: 'hidden', touchAction: 'none' },
  video: { width: '100%', height: '100%', objectFit: 'contain', background: '#000' },
  tapLayer: { position: 'absolute', inset: 0, background: 'transparent', border: 'none' },

  spinner: { position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', color: 'var(--text-muted)', pointerEvents: 'none' },
  error: {
    position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%,-50%)',
    display: 'flex', flexDirection: 'column', alignItems: 'center',
    background: 'rgba(0,0,0,0.75)', color: 'var(--error)', padding: '1.5rem 2rem',
    borderRadius: 'var(--radius-md)', textAlign: 'center',
  },

  controls: {
    position: 'absolute', inset: 0,
    display: 'flex', flexDirection: 'column', justifyContent: 'space-between',
    transition: 'opacity 200ms ease',
    background: 'linear-gradient(to bottom, rgba(0,0,0,0.55) 0%, transparent 25%, transparent 60%, rgba(0,0,0,0.75) 100%)',
  },
  topBar: {
    display: 'flex', alignItems: 'center', gap: '0.75rem',
    padding: 'calc(0.75rem + env(safe-area-inset-top,0px)) calc(0.75rem + env(safe-area-inset-right,0px)) 0.75rem calc(0.75rem + env(safe-area-inset-left,0px))',
  },
  titleWrap: { flex: 1, minWidth: 0 },
  title: { fontSize: '1.05rem', fontWeight: 700, color: '#fff', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  subtitle: { fontSize: '0.8rem', color: 'rgba(255,255,255,0.7)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },

  center: { display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '1.5rem' },
  sideBtn: {
    width: '3rem', height: '3rem', display: 'grid', placeItems: 'center',
    fontSize: '1.6rem', color: '#fff', background: 'transparent',
  },
  playBtn: {
    width: '4.5rem', height: '4.5rem', display: 'grid', placeItems: 'center',
    fontSize: '2.4rem', color: '#fff',
    background: 'rgba(255,255,255,0.16)', borderRadius: '50%', backdropFilter: 'blur(8px)',
  },

  bottom: {
    display: 'flex', flexDirection: 'column', gap: '0.4rem',
    padding: '0 calc(1rem + env(safe-area-inset-right,0px)) calc(1rem + env(safe-area-inset-bottom,0px)) calc(1rem + env(safe-area-inset-left,0px))',
  },
  timeRow: { display: 'flex', justifyContent: 'space-between', color: 'rgba(255,255,255,0.85)', fontSize: '0.8rem', fontVariantNumeric: 'tabular-nums' },
  // Tall invisible hit area so the thin visual track is easy to grab.
  barHit: { padding: '0.75rem 0', cursor: 'pointer', touchAction: 'none' },
  barTrack: { position: 'relative', height: '0.3rem', background: 'rgba(255,255,255,0.25)', borderRadius: '999px' },
  barBuffered: { position: 'absolute', top: 0, left: 0, height: '100%', background: 'rgba(255,255,255,0.4)', borderRadius: '999px' },
  barFill: { position: 'absolute', top: 0, left: 0, height: '100%', background: 'var(--accent)', borderRadius: '999px' },
  barThumb: {
    position: 'absolute', top: '50%', width: '1rem', height: '1rem',
    marginLeft: '-0.5rem', transform: 'translateY(-50%)',
    borderRadius: '50%', background: 'var(--accent)', boxShadow: '0 0 0 0.2rem rgba(0,0,0,0.4)',
  },
  trackRow: { display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '0.25rem' },

  iconBtn: {
    width: '2.75rem', height: '2.75rem', display: 'grid', placeItems: 'center',
    fontSize: '1.4rem', color: '#fff', background: 'transparent', borderRadius: 'var(--radius-md)',
  },
  iconBtnActive: { color: 'var(--accent)' },

  upNext: {
    position: 'absolute', right: '1rem', bottom: '7rem',
    display: 'flex', gap: '0.75rem', alignItems: 'center', textAlign: 'left',
    maxWidth: '20rem', padding: '0.6rem', paddingRight: '1rem',
    background: 'var(--bg-elev)', borderRadius: 'var(--radius-lg)',
    boxShadow: '0 0.75rem 2rem rgba(0,0,0,0.6)',
  },
  upNextThumb: { width: '5rem', height: '3rem', objectFit: 'cover', borderRadius: 'var(--radius-sm)', flex: '0 0 auto' },
  upNextInfo: { display: 'flex', flexDirection: 'column', minWidth: 0 },
  upNextLabel: { fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)' },
  upNextTitle: { fontSize: '0.95rem', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  upNextSubtitle: { fontSize: '0.8rem', color: 'var(--text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },

  time: {},
}
