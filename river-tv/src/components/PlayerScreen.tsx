import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { RiForward10Line, RiReplay10Line } from 'react-icons/ri'
import { api, type AudioTrack, type Subtitle } from '../api'
import {
  FocusProvider,
  useFocusContext,
  useFocusable,
} from '../hooks/useFocus'
import { Popup } from './Popup'
import {
  MAX_ZOOM,
  MIN_ZOOM,
  useAspectRatio,
  type FitMode,
} from '../hooks/useAspectRatio'
import { imageUrl } from '../util/imageUrl'

/*
 * Shared video player used by MoviePlayerPage and EpisodePlayerPage.
 *
 * Inputs are intentionally generic — caller supplies the stream URL,
 * title, progress identity (kind + id), and optional subtitle / audio
 * fetchers + "up next" descriptor. The component owns:
 *
 *   • <video> with autoplay, resume from saved position, ended-handler
 *   • progress reporting over the WebSocket (throttled + final flush)
 *   • on-screen controls (skip ±10s, play/pause, audio, subs) that
 *     auto-hide after 4 s and reappear on any input
 *   • buffered-range indicator on the seek bar
 *   • seek-feedback pill (e.g. "+10s") when scrubbing
 *   • subtitle picker (HTML <track> mode toggling) and audio-track
 *     picker (mute video + sync a separate <audio> element)
 *   • "Up Next" card during the last 30 s of the current item
 *
 * Key handling: FocusProvider manages D-pad nav between visible control
 * buttons. A second window-level listener catches arrow / enter / back
 * presses while the controls are hidden — that listener no-ops when
 * controls are visible so the two never both react to the same key.
 */

const CONTROLS_HIDE_MS = 4000
const PROGRESS_SEND_INTERVAL_S = 5
const SEEK_SECONDS = 10
const RESUME_TAIL_GUARD_S = 30
const UP_NEXT_THRESHOLD_S = 30
const SEEK_PILL_HIDE_MS = 800
const AUDIO_SYNC_DRIFT_S = 0.15

export interface UpNext {
  title: string
  subtitle?: string
  posterUrl?: string
  onPlay: () => void
}

interface Props {
  streamUrl: string
  title: string
  subtitle?: string
  progressKind: 'movie' | 'episode' | 'chapter'
  progressId: string
  fetchSubtitles?: () => Promise<Subtitle[]>
  fetchAudioTracks?: () => Promise<AudioTrack[]>
  upNext?: UpNext
  // Skip-back falls through to "restart at 0" when the current playhead
  // is past the threshold below; only when the user hits Skip-back near
  // the start of the file does it jump to the previous item.
  onPrev?: () => void
  onNext?: () => void
  // When true, skip the resume-from-saved-position step and start at 0.
  // Used by episode skip-next / up-next flows: the user just chose to
  // play this episode, dropping them back into where they paused it the
  // last time would feel like a bug.
  startFromBeginning?: boolean
  // Audio-only mode: the file plays via the <video> element (which is
  // happy with audio-only media), but a cover image fills the screen
  // in place of the video frame, and the aspect / subtitle / audio-
  // track controls hide since they don't apply.
  audioOnly?: boolean
  coverUrl?: string
  onExit: () => void
}

// Seconds — under this point the Skip-back button jumps to the previous
// item (if available); past it, Skip-back just restarts the current.
const SKIP_PREV_THRESHOLD_S = 10

export function PlayerScreen(props: Props) {
  return (
    <FocusProvider onBack={props.onExit}>
      <PlayerInner {...props} />
    </FocusProvider>
  )
}

function PlayerInner({
  streamUrl, title, subtitle,
  progressKind, progressId,
  fetchSubtitles, fetchAudioTracks,
  upNext, onPrev, onNext, startFromBeginning,
  audioOnly, coverUrl, onExit,
}: Props) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const audioRef = useRef<HTMLAudioElement>(null)

  const [paused, setPaused] = useState(true)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [bufferedEnd, setBufferedEnd] = useState(0)
  const [buffering, setBuffering] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showControls, setShowControls] = useState(true)
  const [seekPill, setSeekPill] = useState<string | null>(null)

  const [subtitles, setSubtitles] = useState<Subtitle[]>([])
  const [audioTracks, setAudioTracks] = useState<AudioTrack[]>([])
  // null = no subtitles shown; otherwise id of the selected track.
  const [activeSubtitleId, setActiveSubtitleId] = useState<string | null>(null)
  // null = use the video's built-in audio; otherwise id of an alt track.
  const [activeAudioId, setActiveAudioId] = useState<string | null>(null)

  const [showSubsPicker, setShowSubsPicker] = useState(false)
  const [showAudioPicker, setShowAudioPicker] = useState(false)
  const [showAspectPicker, setShowAspectPicker] = useState(false)
  const [upNextDismissed, setUpNextDismissed] = useState(false)

  const { fitMode, zoom, setFitMode, zoomIn, zoomOut, reset: resetAspect } = useAspectRatio()

  const hideTimer = useRef<number | null>(null)
  const seekPillTimer = useRef<number | null>(null)

  const bumpControls = useCallback(() => {
    setShowControls(true)
    if (hideTimer.current) window.clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => setShowControls(false), CONTROLS_HIDE_MS)
  }, [])

  const showSeekPill = useCallback((deltaS: number) => {
    setSeekPill(deltaS > 0 ? `+${deltaS}s` : `${deltaS}s`)
    if (seekPillTimer.current) window.clearTimeout(seekPillTimer.current)
    seekPillTimer.current = window.setTimeout(() => setSeekPill(null), SEEK_PILL_HIDE_MS)
  }, [])

  // Pause the FocusProvider while controls are hidden. The window
  // listener takes over and treats arrows as scrub instead of nav.
  const ctx = useFocusContext()
  useEffect(() => {
    ctx?.setPaused(!showControls)
    // Snap focus back to the seek bar each time the controls come
    // back into view. Without this, the previously-focused element
    // (e.g. the audio button) would still hold focus, and the user
    // would have to navigate back to the bar to scrub.
    if (showControls) ctx?.focusByTag('seekbar')
  }, [ctx, showControls])

  // Initial controls timer.
  useEffect(() => {
    bumpControls()
    return () => {
      if (hideTimer.current) window.clearTimeout(hideTimer.current)
      if (seekPillTimer.current) window.clearTimeout(seekPillTimer.current)
    }
  }, [bumpControls])

  // Resume from saved position. Only applies if we're meaningfully into
  // the file and not in the last 30s. Skipped entirely when the caller
  // asked to start from the beginning (skip-next, up-next).
  useEffect(() => {
    if (!progressId || startFromBeginning) return
    let resumed = false
    api.getProgress(progressKind, progressId).then(p => {
      const v = videoRef.current
      if (resumed || !v || !p) return
      const apply = () => {
        if (p.position > 5 && p.duration > 0 && p.position < p.duration - RESUME_TAIL_GUARD_S) {
          v.currentTime = p.position
        }
        resumed = true
      }
      if (v.readyState >= 1) apply()
      else v.addEventListener('loadedmetadata', apply, { once: true })
    }).catch(() => {})
  }, [progressKind, progressId, startFromBeginning])

  // Throttled progress reporting + final flush on unmount.
  useEffect(() => {
    if (!progressId) return
    const sock = api.openProgressSocket()
    let lastSent = -PROGRESS_SEND_INTERVAL_S
    const tick = () => {
      const v = videoRef.current
      if (!v || v.paused || !v.duration) return
      if (Math.abs(v.currentTime - lastSent) >= PROGRESS_SEND_INTERVAL_S) {
        sock.send(progressKind, progressId, v.currentTime, v.duration)
        lastSent = v.currentTime
      }
    }
    const interval = window.setInterval(tick, 1000)
    return () => {
      const v = videoRef.current
      if (v && v.duration) sock.send(progressKind, progressId, v.currentTime, v.duration)
      window.clearInterval(interval)
      sock.close()
    }
  }, [progressKind, progressId])

  // Load subtitle + audio track metadata once.
  useEffect(() => {
    let alive = true
    if (fetchSubtitles) fetchSubtitles().then(s => { if (alive) setSubtitles(s) }).catch(() => {})
    if (fetchAudioTracks) fetchAudioTracks().then(a => { if (alive) setAudioTracks(a) }).catch(() => {})
    return () => { alive = false }
  }, [fetchSubtitles, fetchAudioTracks])

  // Toggle subtitle TextTrack modes when the active track changes.
  useEffect(() => {
    const v = videoRef.current
    if (!v) return
    const idx = activeSubtitleId ? subtitles.findIndex(s => s.id === activeSubtitleId) : -1
    for (let i = 0; i < v.textTracks.length; i++) {
      v.textTracks[i].mode = i === idx ? 'showing' : 'hidden'
    }
  }, [activeSubtitleId, subtitles])

  // Alt-audio sync: mirror video state to the separate <audio> element
  // when an alternate track is selected. The default-audio path keeps
  // the <audio> element unmounted so there's no overhead when not used.
  const altAudio = activeAudioId !== null
  useEffect(() => {
    if (!altAudio) return
    const v = videoRef.current
    const a = audioRef.current
    if (!v || !a) return
    const onPlay = () => { void a.play() }
    const onPauseEv = () => { a.pause() }
    const onSeeked = () => { a.currentTime = v.currentTime }
    const onTimeUpdate = () => {
      if (Math.abs(a.currentTime - v.currentTime) > AUDIO_SYNC_DRIFT_S) {
        a.currentTime = v.currentTime
      }
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

  const togglePlay = useCallback(() => {
    const v = videoRef.current
    if (!v) return
    if (v.paused) void v.play()
    else v.pause()
    bumpControls()
  }, [bumpControls])

  // Skip-back: near the start of the file → previous item (if any),
  // otherwise restart at zero. Works for movies too — they just don't
  // have an onPrev so it always restarts.
  const skipBack = useCallback(() => {
    const v = videoRef.current
    if (!v) return
    if (v.currentTime < SKIP_PREV_THRESHOLD_S && onPrev) onPrev()
    else v.currentTime = 0
    bumpControls()
  }, [bumpControls, onPrev])

  const seekBy = useCallback((deltaS: number) => {
    const v = videoRef.current
    if (!v) return
    const next = Math.max(0, Math.min((v.duration || 0), v.currentTime + deltaS))
    v.currentTime = next
    showSeekPill(deltaS)
    bumpControls()
  }, [bumpControls, showSeekPill])

  // Window-level handler: only does anything while controls are hidden
  // (FocusProvider drives D-pad nav while controls are visible).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (showControls) {
        // Any key while controls are visible just refreshes the timer
        // — actual key handling is FocusProvider's job.
        bumpControls()
        return
      }
      switch (e.key) {
        case 'Enter':
        case ' ':
          e.preventDefault()
          togglePlay()
          break
        case 'ArrowLeft':
          e.preventDefault()
          seekBy(-SEEK_SECONDS)
          break
        case 'ArrowRight':
          e.preventDefault()
          seekBy(SEEK_SECONDS)
          break
        case 'ArrowUp':
        case 'ArrowDown':
          // Any other input just brings the controls back so the user
          // can find what they're looking for.
          e.preventDefault()
          bumpControls()
          break
        case 'Escape':
        case 'Backspace':
        case 'GoBack':
          e.preventDefault()
          onExit()
          break
        default:
          bumpControls()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [showControls, bumpControls, togglePlay, seekBy, onExit])

  const remaining = Math.max(0, duration - currentTime)
  const progressPct = duration > 0 ? (currentTime / duration) * 100 : 0
  const bufferedPct = duration > 0 ? (bufferedEnd / duration) * 100 : 0

  const showUpNext =
    !!upNext && !upNextDismissed && duration > 0 && remaining < UP_NEXT_THRESHOLD_S

  const altTrack = altAudio ? audioTracks.find(t => t.id === activeAudioId) ?? null : null

  return (
    <div style={styles.page}>
      <video
        ref={videoRef}
        src={streamUrl}
        autoPlay
        playsInline
        muted={altAudio}
        style={{
          ...styles.video,
          objectFit: fitMode,
          transform: `scale(${zoom})`,
          transformOrigin: 'center center',
        }}
        crossOrigin="anonymous"
        onPlay={() => setPaused(false)}
        onPause={() => setPaused(true)}
        onWaiting={() => setBuffering(true)}
        onPlaying={() => setBuffering(false)}
        onCanPlay={() => setBuffering(false)}
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
        // End of file: auto-advance to the next item if there is one
        // (episode → next episode, chapter → next chapter); for movies
        // and the last episode/chapter, fall back to exiting the
        // player.
        onEnded={() => (onNext ? onNext() : onExit())}
        onError={() => setError('Playback failed')}
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

      {altTrack && (
        <audio
          ref={audioRef}
          src={api.audioTrackStreamUrl(altTrack.id)}
          autoPlay
          preload="auto"
          style={{ display: 'none' }}
        />
      )}

      {audioOnly && (
        <div style={styles.coverOverlay}>
          {imageUrl(coverUrl) ? (
            <img src={imageUrl(coverUrl)} alt={title} decoding="async" style={styles.coverImage} />
          ) : (
            <div style={styles.coverFallback}>{title.charAt(0) || '♪'}</div>
          )}
        </div>
      )}

      {buffering && !error && <div style={styles.spinner}>Loading…</div>}

      {error && (
        <div style={styles.error}>
          {error}
          <div style={styles.errorHint}>Press Back to return.</div>
        </div>
      )}

      {seekPill && <div style={styles.seekPill}>{seekPill}</div>}

      {showUpNext && upNext && (
        <UpNextCard
          item={upNext}
          onDismiss={() => setUpNextDismissed(true)}
        />
      )}

      <div style={{ ...styles.controls, opacity: showControls ? 1 : 0 }}>
        <div style={styles.controlsBg} />
        <div style={styles.controlsContent}>
          <div style={styles.titleRow}>
            <div style={styles.title}>{title}</div>
            {subtitle && <div style={styles.subtitle}>{subtitle}</div>}
          </div>

          <SeekBar
            progressPct={progressPct}
            bufferedPct={bufferedPct}
            autoFocus
            onSeekBack={() => seekBy(-SEEK_SECONDS)}
            onSeekForward={() => seekBy(SEEK_SECONDS)}
            onTogglePlay={togglePlay}
          />

          <div style={styles.bottomRow}>
            <div style={styles.leftGroup}>
              {onPrev && (
                <CtrlButton
                  label="⏮"
                  ariaLabel="Restart or previous"
                  overrides={{ up: 'seekbar' }}
                  onSelect={skipBack}
                />
              )}
              <CtrlButton
                label={<RiReplay10Line />}
                ariaLabel={`Rewind ${SEEK_SECONDS} seconds`}
                overrides={{ up: 'seekbar' }}
                onSelect={() => seekBy(-SEEK_SECONDS)}
              />
              <CtrlButton
                label={paused ? '▶' : '❚❚'}
                ariaLabel={paused ? 'Play' : 'Pause'}
                primary
                tag="playpause"
                overrides={{ up: 'seekbar' }}
                onSelect={togglePlay}
              />
              <CtrlButton
                label={<RiForward10Line />}
                ariaLabel={`Fast forward ${SEEK_SECONDS} seconds`}
                overrides={{ up: 'seekbar' }}
                onSelect={() => seekBy(SEEK_SECONDS)}
              />
              {onNext && (
                <CtrlButton
                  label="⏭"
                  ariaLabel="Next"
                  overrides={{ up: 'seekbar' }}
                  onSelect={onNext}
                />
              )}
              <span style={styles.time}>{formatTime(currentTime)}</span>
            </div>
            <div style={styles.rightGroup}>
              <span style={styles.time}>-{formatTime(remaining)}</span>
              {!audioOnly && audioTracks.length > 0 && (
                <CtrlButton
                  label="♪"
                  ariaLabel="Audio track"
                  overrides={{ up: 'seekbar' }}
                  onSelect={() => setShowAudioPicker(true)}
                />
              )}
              {!audioOnly && subtitles.length > 0 && (
                <CtrlButton
                  label="CC"
                  ariaLabel="Subtitles"
                  overrides={{ up: 'seekbar' }}
                  onSelect={() => setShowSubsPicker(true)}
                />
              )}
              {!audioOnly && (
                <CtrlButton
                  label="⛶"
                  ariaLabel="Display options"
                  overrides={{ up: 'seekbar' }}
                  onSelect={() => setShowAspectPicker(true)}
                />
              )}
            </div>
          </div>
        </div>
      </div>

      {showSubsPicker && (
        <Popup onClose={() => setShowSubsPicker(false)}>
          <h3 style={styles.popupTitle}>Subtitles</h3>
          <div style={styles.popupList}>
            <PickerRow
              label="Off"
              active={activeSubtitleId === null}
              onSelect={() => { setActiveSubtitleId(null); setShowSubsPicker(false) }}
            />
            {subtitles.map(s => (
              <PickerRow
                key={s.id}
                label={s.label || s.language}
                active={activeSubtitleId === s.id}
                onSelect={() => { setActiveSubtitleId(s.id); setShowSubsPicker(false) }}
              />
            ))}
          </div>
        </Popup>
      )}

      {showAudioPicker && (
        <Popup onClose={() => setShowAudioPicker(false)}>
          <h3 style={styles.popupTitle}>Audio</h3>
          <div style={styles.popupList}>
            <PickerRow
              label="Default (in-file)"
              active={activeAudioId === null}
              onSelect={() => { setActiveAudioId(null); setShowAudioPicker(false) }}
            />
            {audioTracks.map(t => (
              <PickerRow
                key={t.id}
                label={t.label || t.language || `Track ${t.stream_index}`}
                active={activeAudioId === t.id}
                onSelect={() => { setActiveAudioId(t.id); setShowAudioPicker(false) }}
              />
            ))}
          </div>
        </Popup>
      )}

      {showAspectPicker && (
        <Popup onClose={() => setShowAspectPicker(false)}>
          <h3 style={styles.popupTitle}>Display</h3>
          <div style={styles.popupList}>
            {FIT_OPTIONS.map(opt => (
              <PickerRow
                key={opt.mode}
                label={opt.label}
                active={fitMode === opt.mode}
                onSelect={() => setFitMode(opt.mode)}
              />
            ))}
            <div style={styles.zoomRow}>
              <span style={styles.zoomLabel}>Zoom</span>
              <div style={styles.zoomControls}>
                <ZoomButton
                  label="−"
                  disabled={zoom <= MIN_ZOOM + 0.001}
                  onSelect={zoomOut}
                />
                <span style={styles.zoomValue}>{Math.round(zoom * 100)}%</span>
                <ZoomButton
                  label="+"
                  disabled={zoom >= MAX_ZOOM - 0.001}
                  onSelect={zoomIn}
                />
              </div>
            </div>
            <PickerRow
              label="Reset to defaults"
              active={false}
              onSelect={() => { resetAspect(); setShowAspectPicker(false) }}
            />
          </div>
        </Popup>
      )}
    </div>
  )
}

const FIT_OPTIONS: { mode: FitMode; label: string }[] = [
  { mode: 'contain', label: 'Fit (original)' },
  { mode: 'cover',   label: 'Crop / Zoom fill' },
  { mode: 'fill',    label: 'Stretch' },
]

function ZoomButton({
  label, disabled, onSelect,
}: {
  label: string
  disabled: boolean
  onSelect: () => void
}) {
  const ref = useFocusable<HTMLButtonElement>(disabled ? undefined : onSelect)
  return (
    <button
      ref={ref}
      tabIndex={-1}
      disabled={disabled}
      onClick={disabled ? undefined : onSelect}
      style={{
        ...styles.zoomBtn,
        ...(disabled ? styles.zoomBtnDisabled : {}),
      }}
    >
      {label}
    </button>
  )
}

function SeekBar({
  progressPct, bufferedPct, autoFocus,
  onSeekBack, onSeekForward, onTogglePlay,
}: {
  progressPct: number
  bufferedPct: number
  autoFocus?: boolean
  onSeekBack: () => void
  onSeekForward: () => void
  onTogglePlay: () => void
}) {
  // Enter / OK toggles play/pause — useFocusable's first argument IS
  // the onSelect callback that the FocusProvider fires on Enter when
  // this element is focused. Left/Right scrub via customDirections
  // instead of moving focus to a sibling control.
  // tag: 'seekbar' lets the parent re-focus the bar every time the
  // controls come back into view (see the focusByTag effect above), and
  // is the override target the control row buttons use for Up.
  // overrides.down → 'playpause' so Down deterministically lands on the
  // play/pause button rather than whichever control happens to be
  // geometrically closest.
  const ref = useFocusable<HTMLDivElement>(onTogglePlay, {
    autoFocus,
    tag: 'seekbar',
    customDirections: { left: onSeekBack, right: onSeekForward },
    overrides: { down: 'playpause' },
  })
  return (
    <div
      ref={ref}
      tabIndex={-1}
      role="slider"
      aria-valuenow={progressPct}
      onClick={onTogglePlay}
      style={styles.progressBar}
    >
      <div style={{ ...styles.progressBuffered, width: `${bufferedPct}%` }} />
      <div style={{ ...styles.progressFill, width: `${progressPct}%` }} />
      <div style={{ ...styles.progressThumb, left: `${progressPct}%` }} />
    </div>
  )
}

function CtrlButton({
  label, ariaLabel, onSelect, primary, autoFocus, tag, overrides,
}: {
  label: ReactNode
  ariaLabel?: string
  onSelect: () => void
  primary?: boolean
  autoFocus?: boolean
  tag?: string
  overrides?: { up?: string; down?: string; left?: string; right?: string }
}) {
  const ref = useFocusable<HTMLButtonElement>(onSelect, { autoFocus, tag, overrides })
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      aria-label={ariaLabel ?? (typeof label === 'string' ? label : undefined)}
      style={{ ...styles.ctrlBtn, ...(primary ? styles.ctrlBtnPrimary : {}) }}
    >
      {label}
    </button>
  )
}

function PickerRow({
  label, active, onSelect,
}: {
  label: string
  active: boolean
  onSelect: () => void
}) {
  const ref = useFocusable<HTMLButtonElement>(onSelect)
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={{ ...styles.popupRow, ...(active ? styles.popupRowActive : {}) }}
    >
      <span>{label}</span>
      {active && <span style={styles.popupRowCheck}>✓</span>}
    </button>
  )
}

function UpNextCard({
  item, onDismiss,
}: {
  item: UpNext
  onDismiss: () => void
}) {
  // autoFocus so the user can hit OK to advance without aiming the
  // D-pad. The card disappears once dismissed (which the parent does
  // after the user activates it).
  const ref = useFocusable<HTMLButtonElement>(
    () => { onDismiss(); item.onPlay() },
    { autoFocus: true },
  )
  return (
    <div style={styles.upNext}>
      {imageUrl(item.posterUrl) && (
        <img src={imageUrl(item.posterUrl)} alt="" decoding="async" style={styles.upNextThumb} />
      )}
      <div style={styles.upNextInfo}>
        <div style={styles.upNextLabel}>Up Next</div>
        <div style={styles.upNextTitle}>{item.title}</div>
        {item.subtitle && <div style={styles.upNextSubtitle}>{item.subtitle}</div>}
        <button
          ref={ref}
          tabIndex={-1}
          onClick={() => { onDismiss(); item.onPlay() }}
          style={styles.upNextBtn}
        >
          ▶ Play Now
        </button>
      </div>
    </div>
  )
}

function formatTime(s: number): string {
  if (!isFinite(s) || s < 0) return '0:00'
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = Math.floor(s % 60)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${m}:${pad(sec)}`
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    position: 'fixed',
    inset: 0,
    background: '#000',
    overflow: 'hidden',
  },
  video: {
    width: '100%',
    height: '100%',
    objectFit: 'contain',
    background: '#000',
  },
  coverOverlay: {
    position: 'absolute',
    inset: 0,
    display: 'grid',
    placeItems: 'center',
    background: '#000',
    // pointer-events: none so the cover doesn't intercept clicks meant
    // for the controls overlay above it.
    pointerEvents: 'none',
  },
  coverImage: {
    maxWidth: 'min(70vmin, 30rem)',
    maxHeight: 'min(70vmin, 30rem)',
    aspectRatio: '1 / 1',
    objectFit: 'contain',
    borderRadius: 'var(--radius-lg)',
    boxShadow: '0 1.5rem 4rem rgba(0,0,0,0.6)',
  },
  coverFallback: {
    width: 'min(70vmin, 30rem)',
    height: 'min(70vmin, 30rem)',
    display: 'grid',
    placeItems: 'center',
    fontSize: '8rem',
    fontWeight: 700,
    color: 'var(--text-muted)',
    background: 'var(--bg-elev-2)',
    borderRadius: 'var(--radius-lg)',
  },

  spinner: {
    position: 'absolute',
    inset: 0,
    display: 'grid',
    placeItems: 'center',
    color: 'var(--text-muted)',
    fontSize: '1.1rem',
    pointerEvents: 'none',
  },
  error: {
    position: 'absolute',
    top: '50%',
    left: '50%',
    transform: 'translate(-50%, -50%)',
    background: 'rgba(0,0,0,0.7)',
    color: 'var(--error)',
    padding: '1.5rem 2rem',
    borderRadius: 'var(--radius-md)',
    textAlign: 'center',
  },
  errorHint: {
    marginTop: '0.5rem',
    fontSize: '0.9rem',
    color: 'var(--text-muted)',
  },
  seekPill: {
    position: 'absolute',
    top: '50%',
    left: '50%',
    transform: 'translate(-50%, -50%)',
    background: 'rgba(0,0,0,0.7)',
    color: 'var(--text)',
    padding: '1rem 1.5rem',
    borderRadius: 'var(--radius-md)',
    fontSize: '1.5rem',
    fontWeight: 700,
    pointerEvents: 'none',
  },

  controls: {
    position: 'absolute',
    left: 0,
    right: 0,
    bottom: 0,
    transition: 'opacity 250ms ease',
  },
  controlsBg: {
    position: 'absolute',
    inset: 0,
    background: 'linear-gradient(to top, rgba(0,0,0,0.9) 0%, rgba(0,0,0,0.5) 70%, transparent 100%)',
    pointerEvents: 'none',
  },
  controlsContent: {
    position: 'relative',
    padding: 'var(--safe-y) var(--safe-x)',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.75rem',
  },
  titleRow: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
  },
  title: {
    fontSize: '1.5rem',
    fontWeight: 700,
    color: 'var(--text)',
  },
  subtitle: {
    fontSize: '0.95rem',
    color: 'var(--text-muted)',
  },
  progressBar: {
    position: 'relative',
    height: '0.5rem',
    background: 'rgba(255,255,255,0.2)',
    borderRadius: '999px',
    // Don't clip the thumb — it sits above the track.
    overflow: 'visible',
  },
  progressBuffered: {
    position: 'absolute',
    inset: 0,
    height: '100%',
    background: 'rgba(255,255,255,0.35)',
    borderRadius: '999px',
    transition: 'width 250ms linear',
  },
  progressFill: {
    position: 'absolute',
    inset: 0,
    height: '100%',
    background: 'var(--accent)',
    borderRadius: '999px',
    transition: 'width 250ms linear',
  },
  progressThumb: {
    position: 'absolute',
    top: '50%',
    width: '1.1rem',
    height: '1.1rem',
    marginLeft: '-0.55rem',
    transform: 'translateY(-50%)',
    borderRadius: '50%',
    background: 'var(--accent)',
    boxShadow: '0 0 0 0.25rem var(--bg)',
    transition: 'left 250ms linear',
  },
  bottomRow: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: '1.5rem',
    color: 'var(--text)',
    fontVariantNumeric: 'tabular-nums',
  },
  leftGroup: {
    display: 'flex',
    alignItems: 'center',
    gap: '1rem',
  },
  rightGroup: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.75rem',
  },
  time: {
    flex: '0 0 auto',
    fontSize: '1rem',
    color: 'var(--text-muted)',
  },
  ctrlBtn: {
    // Symbol-driven buttons want a square hit area at TV distance — fix
    // both axes so a wide glyph (♪) sits the same size as a narrow one
    // (CC) and the row stays evenly spaced.
    width: '3.25rem',
    height: '3.25rem',
    display: 'grid',
    placeItems: 'center',
    fontSize: '1.4rem',
    fontWeight: 600,
    color: 'var(--text)',
    background: 'rgba(255,255,255,0.16)',
    borderRadius: 'var(--radius-md)',
    backdropFilter: 'blur(8px)',
    lineHeight: 1,
  },
  ctrlBtnPrimary: {
    background: 'var(--accent)',
    color: 'var(--on-accent)',
    fontSize: '1.5rem',
    width: '4rem',
    height: '4rem',
  },

  upNext: {
    position: 'absolute',
    right: '2rem',
    bottom: '14rem',
    width: '24rem',
    background: 'var(--bg-elev)',
    borderRadius: 'var(--radius-lg)',
    overflow: 'hidden',
    boxShadow: '0 1rem 3rem rgba(0,0,0,0.6)',
    display: 'flex',
    flexDirection: 'column',
  },
  upNextThumb: {
    width: '100%',
    height: '8rem',
    objectFit: 'cover',
  },
  upNextInfo: {
    padding: '1rem 1.25rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.35rem',
  },
  upNextLabel: {
    fontSize: '0.85rem',
    color: 'var(--text-muted)',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
  },
  upNextTitle: {
    fontSize: '1.1rem',
    fontWeight: 700,
    color: 'var(--text)',
  },
  upNextSubtitle: {
    fontSize: '0.9rem',
    color: 'var(--text-muted)',
  },
  upNextBtn: {
    marginTop: '0.75rem',
    padding: '0.7rem 1.25rem',
    fontSize: '1rem',
    fontWeight: 600,
    background: 'var(--accent)',
    color: 'var(--on-accent)',
    borderRadius: 'var(--radius-md)',
  },

  popupTitle: {
    margin: '0 0 1rem',
    fontSize: '1.25rem',
    fontWeight: 600,
    color: 'var(--text-muted)',
  },
  popupList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
  },
  popupRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.9rem 1.25rem',
    fontSize: '1.05rem',
    color: 'var(--text)',
    borderRadius: 'var(--radius-md)',
    background: 'transparent',
    textAlign: 'left',
  },
  popupRowActive: {
    background: 'var(--accent-soft)',
    fontWeight: 600,
  },
  popupRowCheck: {
    color: 'var(--accent)',
    fontSize: '1.1rem',
  },

  zoomRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.9rem 1.25rem',
    marginTop: '0.25rem',
    borderTop: '1px solid rgba(255,255,255,0.08)',
  },
  zoomLabel: {
    fontSize: '1rem',
    color: 'var(--text-muted)',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
  },
  zoomControls: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
  },
  zoomBtn: {
    width: '2.5rem',
    height: '2.5rem',
    fontSize: '1.25rem',
    fontWeight: 700,
    color: 'var(--text)',
    background: 'var(--bg-elev-2)',
    borderRadius: 'var(--radius-md)',
    display: 'grid',
    placeItems: 'center',
  },
  zoomBtnDisabled: {
    opacity: 0.4,
    cursor: 'default',
  },
  zoomValue: {
    minWidth: '4rem',
    textAlign: 'center',
    fontSize: '1.05rem',
    fontVariantNumeric: 'tabular-nums',
    color: 'var(--text)',
  },
}
