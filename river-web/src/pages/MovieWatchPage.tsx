import { useEffect, useRef, useState, useCallback } from 'react'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import {
  RiArrowLeftLine,
  RiPlayFill, RiPauseFill,
  RiVolumeUpLine, RiVolumeDownLine, RiVolumeMuteLine,
  RiFullscreenLine, RiFullscreenExitLine,
  RiClosedCaptioningLine,
  RiSpeakLine,
  RiPictureInPicture2Line,
  RiReplay10Fill, RiForward10Fill,
} from 'react-icons/ri'
import { useMovies } from '../context/MoviesContext'
import { useAuth } from '../context/AuthContext'
import { useWatchParty } from '../hooks/useWatchParty'
import { useCast } from '../hooks/useCast'
import { useAspectRatio, MIN_ZOOM, MAX_ZOOM } from '../hooks/useAspectRatio'
import { WatchPartyOverlay } from '../components/WatchPartyOverlay'
import { CastButton } from '../components/CastButton'
import { AspectRatioMenu } from '../components/AspectRatioMenu'
import { api } from '../api'
import type { Movie, Subtitle, AudioTrack, WatchParty } from '../api'

interface NativeAudioTrack { id: string; label: string; language: string; enabled: boolean }
interface VideoWithAudioTracks extends HTMLVideoElement {
  audioTracks?: { length: number; [i: number]: NativeAudioTrack }
}
import styles from './MovieWatchPage.module.css'

const HIDE_DELAY = 3000
const REPORT_INTERVAL = 10
const SKIP_SECONDS = 10

interface VTTCue { start: number; end: number; text: string }

function parseVTTTime(s: string): number {
  const parts = s.trim().split(':')
  if (parts.length === 3) return +parts[0] * 3600 + +parts[1] * 60 + parseFloat(parts[2])
  return +parts[0] * 60 + parseFloat(parts[1])
}

function parseVTT(content: string): VTTCue[] {
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

export function MovieWatchPage() {
  const { id } = useParams<{ id: string }>()
  const [searchParams] = useSearchParams()
  const partyId = searchParams.get('party') ?? undefined
  const { user } = useAuth()
  const { getOne, streamUrl } = useMovies()
  const [movie, setMovie] = useState<Movie | null>(null)
  const [room, setRoom] = useState<WatchParty | null>(null)
  const isHost = partyId ? room?.host_id === user?.id : true

  const videoRef = useRef<HTMLVideoElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const lastReportRef = useRef(0)
  const pendingSeekRef = useRef<number | null>(null)
  const pendingPlayRef = useRef(false)
  const progressRef = useRef({ position: 0, duration: 0 })
  const subtitleCuesRef = useRef<VTTCue[]>([])
  const wsRef = useRef<ReturnType<typeof api.openProgressSocket> | null>(null)
  const [videoSrc, setVideoSrc] = useState<string | undefined>(() => id ? streamUrl(id) : undefined)

  const [playing, setPlaying] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(1)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [fullscreen, setFullscreen] = useState(false)
  const [pip, setPip] = useState(false)
  const [controlsVisible, setControlsVisible] = useState(true)
  const [seeking, setSeeking] = useState(false)
  const [subtitles, setSubtitles] = useState<Subtitle[]>([])
  const [subMenuOpen, setSubMenuOpen] = useState(false)
  const [activeSubtitleId, setActiveSubtitleId] = useState<string | null>(null)
  const [subtitleText, setSubtitleText] = useState('')
  const [audioTracks, setAudioTracks] = useState<AudioTrack[]>([])
  const [audioMenuOpen, setAudioMenuOpen] = useState(false)
  const [activeAudioIdx, setActiveAudioIdx] = useState(0)
  const [aspectMenuOpen, setAspectMenuOpen] = useState(false)
  const aspect = useAspectRatio()

  useEffect(() => {
    if (!id) return
    getOne(id).then(setMovie).catch(() => {})
    api.getMovieSubtitles(id).then(setSubtitles).catch(() => {})
    api.getMovieAudioTracks(id).then(setAudioTracks).catch(() => {})
    wsRef.current = api.openProgressSocket()
    return () => { wsRef.current?.close(); wsRef.current = null }
  }, [id, getOne])

  useEffect(() => {
    if (!partyId) return
    api.getWatchParty(partyId).then(setRoom).catch(() => {})
  }, [partyId])

  const { members, sendCommand } = useWatchParty(partyId, videoRef, isHost, `/movie/${id}`)
  const { castReady, isCasting, loadCastMedia } = useCast()

  useEffect(() => {
    if (!isCasting || !movie || !videoSrc) return
    const meta = new chrome.cast.media.MovieMediaMetadata()
    meta.title = movie.title
    if (movie.poster_path) meta.images = [{ url: movie.poster_path }]
    loadCastMedia(videoSrc, 'video/mp4', meta, videoRef.current?.currentTime)
  }, [isCasting, movie, videoSrc, loadCastMedia])

  // Fetch and parse VTT when active subtitle changes
  useEffect(() => {
    if (!activeSubtitleId) {
      subtitleCuesRef.current = []
      // eslint-disable-next-line react-hooks/set-state-in-effect -- clears rendered subtitle text when the active subtitle is turned off
      setSubtitleText('')
      return
    }
    fetch(api.subtitleStreamUrl(activeSubtitleId))
      .then(r => r.text())
      .then(content => { subtitleCuesRef.current = parseVTT(content) })
      .catch(() => { subtitleCuesRef.current = [] })
  }, [activeSubtitleId])

  // Fetch saved progress and queue a seek once metadata loads
  useEffect(() => {
    if (!id) return
    api.getProgress('movie', id).then(p => {
      if (!p || p.completed) return
      if (p.position > 5 && (p.duration <= 0 || p.position < p.duration - 30)) {
        if (videoRef.current && videoRef.current.readyState >= 1) {
          videoRef.current.currentTime = p.position
        } else {
          pendingSeekRef.current = p.position
        }
      }
    }).catch(() => {})
  }, [id])

  // Flush final position on unmount
  useEffect(() => {
    return () => {
      const { position, duration: d } = progressRef.current
      if (id && position > 5) {
        wsRef.current?.send('movie', id, position, d)
      }
    }
  }, [id])

  // Auto-hide controls
  const showControls = useCallback(() => {
    setControlsVisible(true)
    if (hideTimerRef.current) clearTimeout(hideTimerRef.current)
    if (playing) {
      hideTimerRef.current = setTimeout(() => setControlsVisible(false), HIDE_DELAY)
    }
  }, [playing])

  useEffect(() => {
    return () => { if (hideTimerRef.current) clearTimeout(hideTimerRef.current) }
  }, [])

  useEffect(() => {
    if (!playing) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- shows/schedules hiding of the controls overlay in response to play state
      setControlsVisible(true)
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current)
    } else {
      hideTimerRef.current = setTimeout(() => setControlsVisible(false), HIDE_DELAY)
    }
  }, [playing])

  const onPlay = () => {
    setPlaying(true)
    if (partyId && isHost) {
      const v = videoRef.current
      if (v) sendCommand('play', v.currentTime)
    }
  }

  const onPause = () => {
    setPlaying(false)
    const v = videoRef.current
    if (id && v && v.currentTime > 5) {
      wsRef.current?.send('movie', id, v.currentTime, v.duration || 0)
    }
    if (partyId && isHost && v) sendCommand('pause', v.currentTime)
  }

  const onEnded = () => {
    setPlaying(false)
    const v = videoRef.current
    if (id && v) {
      wsRef.current?.send('movie', id, v.currentTime, v.duration || 0)
    }
  }

  const onTimeUpdate = () => {
    const v = videoRef.current
    if (!v || seeking) return
    setCurrentTime(v.currentTime)
    progressRef.current = { position: v.currentTime, duration: v.duration || 0 }
    if (v.currentTime - lastReportRef.current >= REPORT_INTERVAL) {
      lastReportRef.current = v.currentTime
      if (id) wsRef.current?.send('movie', id, v.currentTime, v.duration || 0)
    }
    const cues = subtitleCuesRef.current
    if (cues.length > 0) {
      const t = v.currentTime
      const cue = cues.find(c => t >= c.start && t <= c.end)
      setSubtitleText(cue?.text ?? '')
    } else if (subtitleText) {
      setSubtitleText('')
    }
  }

  const onLoadedMetadata = () => {
    const v = videoRef.current
    if (!v) return
    setDuration(v.duration)
    if (pendingSeekRef.current !== null) {
      v.currentTime = pendingSeekRef.current
      pendingSeekRef.current = null
    }
    if (pendingPlayRef.current) {
      pendingPlayRef.current = false
      v.play().catch(() => {})
    }
  }

  const onVolumeChange = () => {
    const v = videoRef.current
    if (!v) return
    setMuted(v.muted)
    setVolume(v.volume)
  }

  const togglePlay = () => {
    const v = videoRef.current
    if (!v || (partyId && !isHost)) return
    if (v.paused) void v.play()
    else v.pause()
  }

  const toggleMute = () => {
    const v = videoRef.current
    if (!v) return
    v.muted = !v.muted
  }

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = videoRef.current
    if (!v) return
    const val = parseFloat(e.target.value)
    v.volume = val
    v.muted = val === 0
  }

  const skip = (secs: number) => {
    const v = videoRef.current
    if (!v) return
    v.currentTime = Math.max(0, Math.min(v.duration, v.currentTime + secs))
  }

  const handleSeekStart = () => setSeeking(true)

  const handleSeekChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setCurrentTime(parseFloat(e.target.value))
  }

  const handleSeekEnd = (e: React.MouseEvent<HTMLInputElement>) => {
    const v = videoRef.current
    if (!v || (partyId && !isHost)) return
    const newTime = parseFloat((e.target as HTMLInputElement).value)
    v.currentTime = newTime
    setSeeking(false)
    if (partyId && isHost) sendCommand('seek', newTime)
  }

  const switchAudioTrack = (index: number) => {
    const track = audioTracks[index]
    if (!track) return
    setActiveAudioIdx(index)
    setAudioMenuOpen(false)

    const nativeTracks = (videoRef.current as VideoWithAudioTracks | null)?.audioTracks
    if (nativeTracks && nativeTracks.length > 0) {
      for (let i = 0; i < nativeTracks.length; i++) {
        // eslint-disable-next-line react-hooks/immutability -- mutating the native HTMLMediaElement AudioTrackList (a DOM API), not React ref state
        nativeTracks[i].enabled = i === index
      }
    } else {
      const savedTime = videoRef.current?.currentTime ?? 0
      const wasPlaying = !(videoRef.current?.paused ?? true)
      pendingSeekRef.current = savedTime > 1 ? savedTime : null
      pendingPlayRef.current = wasPlaying
      setVideoSrc(api.audioTrackStreamUrl(track.id))
    }
  }

  const toggleFullscreen = () => {
    const el = containerRef.current
    if (!el) return
    if (document.fullscreenElement) void document.exitFullscreen()
    else void el.requestFullscreen()
  }

  const togglePip = () => {
    const v = videoRef.current
    if (!v) return
    if (document.pictureInPictureElement) {
      document.exitPictureInPicture().catch(() => {})
    } else if (document.pictureInPictureEnabled) {
      v.requestPictureInPicture().catch(() => {})
    }
  }

  useEffect(() => {
    const handler = () => setFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', handler)
    return () => document.removeEventListener('fullscreenchange', handler)
  }, [])

  useEffect(() => {
    const v = videoRef.current
    if (!v) return
    const onEnter = () => setPip(true)
    const onLeave = () => setPip(false)
    v.addEventListener('enterpictureinpicture', onEnter)
    v.addEventListener('leavepictureinpicture', onLeave)
    return () => {
      v.removeEventListener('enterpictureinpicture', onEnter)
      v.removeEventListener('leavepictureinpicture', onLeave)
    }
  }, [])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement) return
      const canControl = !partyId || isHost
      switch (e.key) {
        case ' ': case 'k': e.preventDefault(); if (canControl) togglePlay(); break
        case 'f': toggleFullscreen(); break
        case 'p': togglePip(); break
        case 'm': toggleMute(); break
        case 'ArrowLeft': if (canControl) skip(-SKIP_SECONDS); break
        case 'ArrowRight': if (canControl) skip(SKIP_SECONDS); break
        case '+': case '=': e.preventDefault(); aspect.zoomIn(); break
        case '-': case '_': e.preventDefault(); aspect.zoomOut(); break
        case '0': e.preventDefault(); aspect.reset(); break
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [partyId, isHost, aspect]) // eslint-disable-line react-hooks/exhaustive-deps

  const formatTime = (s: number) => {
    if (!isFinite(s)) return '0:00'
    const h = Math.floor(s / 3600)
    const m = Math.floor((s % 3600) / 60)
    const sec = Math.floor(s % 60)
    if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
    return `${m}:${String(sec).padStart(2, '0')}`
  }

  const progress = duration > 0 ? currentTime / duration : 0
  const VolumeIcon = muted || volume === 0 ? RiVolumeMuteLine : volume < 0.5 ? RiVolumeDownLine : RiVolumeUpLine

  return (
    <div
      ref={containerRef}
      className={styles.container}
      onMouseMove={showControls}
      onMouseLeave={() => playing && setControlsVisible(false)}
      onClick={togglePlay}
      onDoubleClick={toggleFullscreen}
      style={{ cursor: controlsVisible ? 'default' : 'none' }}
    >
      <video
        ref={videoRef}
        className={styles.video}
        src={videoSrc}
        style={{ objectFit: aspect.fitMode, transform: `scale(${aspect.zoom})` }}
        onPlay={onPlay}
        onPause={onPause}
        onEnded={onEnded}
        onTimeUpdate={onTimeUpdate}
        onLoadedMetadata={onLoadedMetadata}
        onVolumeChange={onVolumeChange}
        autoPlay
      />

      {subtitleText && (
        <div className={styles.subtitleOverlay}>{subtitleText}</div>
      )}

      {partyId && (
        <WatchPartyOverlay
          roomId={partyId}
          members={members}
          isHost={isHost}
          backPath={`/movie/${id}`}
          onClick={e => e.stopPropagation()}
          onDoubleClick={e => e.stopPropagation()}
        />
      )}

      {/* Top bar */}
      <div
        className={`${styles.topBar} ${controlsVisible ? styles.visible : ''}`}
        onClick={e => e.stopPropagation()}
        onDoubleClick={e => e.stopPropagation()}
      >
        <Link to={`/movie/${id}`} className={`btn btn-icon ${styles.backBtn}`}>
          <RiArrowLeftLine size={20} />
        </Link>
        {movie && <span className={`label-md ${styles.topTitle}`}>{movie.title}</span>}
      </div>

      {/* Bottom controls */}
      <div
        className={`${styles.controls} ${controlsVisible ? styles.visible : ''}`}
        onClick={e => e.stopPropagation()}
        onDoubleClick={e => e.stopPropagation()}
      >
        <div className={styles.seekRow}>
          <input
            type="range" min={0} max={duration || 100} step={0.1} value={currentTime}
            className={styles.seekBar}
            style={{ '--progress': `${progress * 100}%` } as React.CSSProperties}
            onMouseDown={handleSeekStart}
            onChange={handleSeekChange}
            onMouseUp={handleSeekEnd}
            disabled={!!(partyId && !isHost)}
          />
        </div>

        <div className={styles.buttonRow}>
          <div className={styles.left}>
            <button
              className={`btn btn-icon ${styles.controlBtn}`}
              onClick={() => skip(-SKIP_SECONDS)}
              aria-label={`Rewind ${SKIP_SECONDS} seconds`}
              disabled={!!(partyId && !isHost)}
            >
              <RiReplay10Fill size={22} />
            </button>
            <button className={`btn btn-icon ${styles.controlBtn}`} onClick={togglePlay} aria-label={playing ? 'Pause' : 'Play'} disabled={!!(partyId && !isHost)}>
              {playing ? <RiPauseFill size={22} /> : <RiPlayFill size={22} />}
            </button>
            <button
              className={`btn btn-icon ${styles.controlBtn}`}
              onClick={() => skip(SKIP_SECONDS)}
              aria-label={`Fast forward ${SKIP_SECONDS} seconds`}
              disabled={!!(partyId && !isHost)}
            >
              <RiForward10Fill size={22} />
            </button>
            <div className={styles.volumeGroup}>
              <button className={`btn btn-icon ${styles.controlBtn}`} onClick={toggleMute} aria-label="Mute">
                <VolumeIcon size={20} />
              </button>
              <input
                type="range" min={0} max={1} step={0.02} value={muted ? 0 : volume}
                className={`${styles.seekBar} ${styles.volumeBar}`}
                style={{ '--progress': `${(muted ? 0 : volume) * 100}%` } as React.CSSProperties}
                onChange={handleVolumeChange}
              />
            </div>
            <span className={`label-sm ${styles.timeDisplay}`}>
              {formatTime(currentTime)} / {formatTime(duration)}
            </span>
          </div>
          <div className={styles.right}>
            {audioTracks.length > 0 && (
              <div className={styles.subMenu}>
                <button
                  className={`btn btn-icon ${styles.controlBtn}`}
                  onClick={e => { e.stopPropagation(); setAudioMenuOpen(o => !o) }}
                  aria-label="Audio language"
                  title={audioTracks[activeAudioIdx]?.label ?? 'Audio'}
                >
                  <RiSpeakLine size={20} />
                </button>
                {audioMenuOpen && (
                  <div className={styles.subMenuList} onClick={e => e.stopPropagation()}>
                    {audioTracks.map((t, i) => (
                      <button
                        key={t.id}
                        className={`${styles.subMenuOption} ${i === activeAudioIdx ? styles.subMenuOptionActive : ''}`}
                        onClick={() => switchAudioTrack(i)}
                      >
                        {t.label || t.language}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
            {subtitles.length > 0 && (
              <div className={styles.subMenu}>
                <button
                  className={`btn btn-icon ${styles.controlBtn}`}
                  onClick={e => { e.stopPropagation(); setSubMenuOpen(o => !o) }}
                  aria-label="Subtitles"
                >
                  <RiClosedCaptioningLine size={20} />
                </button>
                {subMenuOpen && (
                  <div className={styles.subMenuList} onClick={e => e.stopPropagation()}>
                    <button
                      className={`${styles.subMenuOption} ${activeSubtitleId === null ? styles.subMenuOptionActive : ''}`}
                      onClick={() => { setActiveSubtitleId(null); setSubMenuOpen(false) }}
                    >
                      Off
                    </button>
                    {subtitles.map(sub => (
                      <button
                        key={sub.id}
                        className={`${styles.subMenuOption} ${sub.id === activeSubtitleId ? styles.subMenuOptionActive : ''}`}
                        onClick={() => { setActiveSubtitleId(sub.id); setSubMenuOpen(false) }}
                      >
                        {sub.label || sub.language}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
            <AspectRatioMenu
              open={aspectMenuOpen}
              fitMode={aspect.fitMode}
              zoom={aspect.zoom}
              minZoom={MIN_ZOOM}
              maxZoom={MAX_ZOOM}
              onToggle={() => setAspectMenuOpen(o => !o)}
              onSetFit={m => { aspect.setFitMode(m); setAspectMenuOpen(false) }}
              onZoomIn={aspect.zoomIn}
              onZoomOut={aspect.zoomOut}
              onReset={() => { aspect.reset(); setAspectMenuOpen(false) }}
              styles={styles}
            />
            {document.pictureInPictureEnabled && (
              <button
                className={`btn btn-icon ${styles.controlBtn} ${pip ? styles.controlBtnActive : ''}`}
                onClick={e => { e.stopPropagation(); togglePip() }}
                aria-label="Picture in picture"
              >
                <RiPictureInPicture2Line size={20} />
              </button>
            )}
            {castReady && (
              <CastButton className={styles.castBtn} />
            )}
            <button className={`btn btn-icon ${styles.controlBtn}`} onClick={toggleFullscreen} aria-label="Fullscreen">
              {fullscreen ? <RiFullscreenExitLine size={20} /> : <RiFullscreenLine size={20} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
