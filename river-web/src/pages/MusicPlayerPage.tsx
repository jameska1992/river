import { useEffect, useRef, useState, useCallback } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import {
  RiArrowLeftLine,
  RiPlayFill, RiPauseFill,
  RiSkipBackFill, RiSkipForwardFill,
  RiVolumeUpLine, RiVolumeDownLine, RiVolumeMuteLine,
  RiListUnordered, RiCloseLine, RiCheckLine,
  RiMusicLine,
} from 'react-icons/ri'
import { useMusic } from '../context/MusicContext'
import type { Album, Artist, Track } from '../api'
import { api } from '../api'
import { useCast } from '../hooks/useCast'
import { CastButton } from '../components/CastButton'
import { imageUrl } from '../util/imageUrl'
import styles from './MusicPlayerPage.module.css'

const SPEEDS = [0.75, 1, 1.25, 1.5, 1.75, 2]

export function MusicPlayerPage() {
  const { albumId } = useParams<{ albumId: string }>()
  const [searchParams] = useSearchParams()
  const startTrackId = searchParams.get('track')
  const { getAlbum, fetchTracks, trackStreamUrl } = useMusic()

  const audioRef = useRef<HTMLAudioElement>(null)

  const [album, setAlbum] = useState<Album | null>(null)
  const [artist, setArtist] = useState<Artist | null>(null)
  const [tracks, setTracks] = useState<Track[]>([])
  const [trackIdx, setTrackIdx] = useState(0)
  const [isLoading, setIsLoading] = useState(true)

  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [seeking, setSeeking] = useState(false)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)
  const [speedIdx, setSpeedIdx] = useState(1)
  const [queueOpen, setQueueOpen] = useState(false)

  useEffect(() => {
    if (!albumId) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resets loading state before refetching when the album id changes
    setIsLoading(true)
    getAlbum(albumId)
      .then(async a => {
        setAlbum(a)
        const [t, ar] = await Promise.all([
          fetchTracks(albumId),
          a.artist_id ? api.getArtist(a.artist_id).catch(() => null) : Promise.resolve(null),
        ])
        const sorted = [...t].sort((a, b) => a.disc_number - b.disc_number || a.number - b.number)
        setTracks(sorted)
        setArtist(ar)
        if (startTrackId) {
          const idx = sorted.findIndex(t => t.id === startTrackId)
          if (idx >= 0) setTrackIdx(idx)
        }
      })
      .finally(() => setIsLoading(false))
  }, [albumId, getAlbum, fetchTracks, startTrackId])

  const currentTrack = tracks[trackIdx] ?? null

  const pendingPlayRef = useRef(false)
  const [audioSrc, setAudioSrc] = useState<string | undefined>()

  useEffect(() => {
    if (!currentTrack) return
    pendingPlayRef.current = playing
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resets playback position when the current track changes
    setCurrentTime(0)
    setAudioSrc(trackStreamUrl(currentTrack.id))
  }, [currentTrack?.id]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const a = audioRef.current
    if (!a) return
    a.playbackRate = SPEEDS[speedIdx]
  }, [speedIdx])

  const goToTrack = useCallback((idx: number) => {
    if (idx < 0 || idx >= tracks.length) return
    pendingPlayRef.current = true
    setTrackIdx(idx)
    setQueueOpen(false)
  }, [tracks.length])

  const { castReady, isCasting, loadCastMedia } = useCast()

  useEffect(() => {
    if (!isCasting || !currentTrack || !audioSrc) return
    const meta = new chrome.cast.media.MusicTrackMediaMetadata()
    meta.title = currentTrack.title
    if (artist) meta.artist = artist.name
    if (album) {
      meta.albumName = album.title
      if (album.cover_path) meta.images = [{ url: album.cover_path }]
    }
    loadCastMedia(audioSrc, 'audio/mp4', meta, audioRef.current?.currentTime)
  }, [isCasting, currentTrack, audioSrc, loadCastMedia]) // eslint-disable-line react-hooks/exhaustive-deps

  const onLoadedMetadata = () => {
    const a = audioRef.current
    if (!a) return
    setDuration(a.duration)
    a.playbackRate = SPEEDS[speedIdx]
    if (pendingPlayRef.current) {
      pendingPlayRef.current = false
      a.play().catch(() => {})
    }
  }

  const onTimeUpdate = () => {
    const a = audioRef.current
    if (!a || seeking) return
    setCurrentTime(a.currentTime)
  }

  const onEnded = () => {
    setPlaying(false)
    if (trackIdx < tracks.length - 1) {
      pendingPlayRef.current = true
      setTrackIdx(i => i + 1)
    }
  }

  const onPlay = () => setPlaying(true)
  const onPause = () => setPlaying(false)

  const onVolumeChange = () => {
    const a = audioRef.current
    if (!a) return
    setMuted(a.muted)
    setVolume(a.volume)
  }

  const togglePlay = useCallback(() => {
    const a = audioRef.current
    if (!a) return
    if (a.paused) void a.play()
    else a.pause()
  }, [])

  const toggleMute = useCallback(() => {
    const a = audioRef.current
    if (!a) return
    a.muted = !a.muted
  }, [])

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const a = audioRef.current
    if (!a) return
    const val = parseFloat(e.target.value)
    a.volume = val
    a.muted = val === 0
  }

  const handleSeekStart = () => setSeeking(true)

  const handleSeekChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setCurrentTime(parseFloat(e.target.value))
  }

  const handleSeekCommit = (e: React.PointerEvent<HTMLInputElement>) => {
    const a = audioRef.current
    if (!a) return
    a.currentTime = parseFloat((e.target as HTMLInputElement).value)
    setSeeking(false)
  }

  const cycleSpeed = () => setSpeedIdx(i => (i + 1) % SPEEDS.length)

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement) return
      switch (e.key) {
        case ' ': case 'k': e.preventDefault(); togglePlay(); break
        case 'm': toggleMute(); break
        case 'ArrowLeft': case 'p': case 'P': goToTrack(trackIdx - 1); break
        case 'ArrowRight': case 'n': case 'N': goToTrack(trackIdx + 1); break
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [togglePlay, toggleMute, goToTrack, trackIdx])

  const progress = duration > 0 ? currentTime / duration : 0
  const VolumeIcon = muted || volume === 0 ? RiVolumeMuteLine : volume < 0.5 ? RiVolumeDownLine : RiVolumeUpLine

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.backdrop} />
        <div className={styles.loadingState}>
          <RiMusicLine size={48} style={{ color: 'rgba(255,255,255,0.3)' }} />
        </div>
      </div>
    )
  }

  if (!album || !currentTrack) return null

  return (
    <div className={styles.page}>
      {/* Blurred album cover backdrop */}
      {album.cover_path && (
        <div
          className={styles.backdrop}
          style={{ backgroundImage: `url(${album.cover_path})` }}
        />
      )}
      <div className={styles.backdropOverlay} />

      <audio
        ref={audioRef}
        src={audioSrc}
        onLoadedMetadata={onLoadedMetadata}
        onTimeUpdate={onTimeUpdate}
        onEnded={onEnded}
        onPlay={onPlay}
        onPause={onPause}
        onVolumeChange={onVolumeChange}
      />

      {/* Top bar */}
      <div className={styles.topBar}>
        <Link to={`/album/${albumId}`} className={`btn btn-icon ${styles.topBtn}`}>
          <RiArrowLeftLine size={20} />
        </Link>
        <span className={`label-md ${styles.topTitle}`}>
          {album.title}
        </span>
        <button
          className={`btn btn-icon ${styles.topBtn} ${queueOpen ? styles.topBtnActive : ''}`}
          onClick={() => setQueueOpen(o => !o)}
          aria-label="Track queue"
          title="Track queue"
        >
          <RiListUnordered size={20} />
        </button>
      </div>

      {/* Main player */}
      <main className={styles.player}>
        {/* Album art */}
        <div className={styles.coverWrap}>
          {album.cover_path ? (
            <img src={imageUrl(album.cover_path)} alt={album.title} className={styles.cover} />
          ) : (
            <div className={styles.coverFallback}>
              <RiMusicLine size={64} />
            </div>
          )}
        </div>

        {/* Track info */}
        <div className={styles.info}>
          <p className={`headline-sm ${styles.infoTitle}`}>{currentTrack.title}</p>
          {artist && <p className={`label-md ${styles.infoArtist}`}>{artist.name}</p>}
          <p className={`label-sm ${styles.infoAlbum}`}>{album.title}</p>
        </div>

        {/* Seek bar */}
        <div className={styles.seekWrap}>
          <input
            type="range"
            min={0}
            max={duration || 100}
            step={0.1}
            value={currentTime}
            className={styles.seekBar}
            style={{ '--progress': `${progress * 100}%` } as React.CSSProperties}
            onMouseDown={handleSeekStart}
            onPointerDown={handleSeekStart}
            onChange={handleSeekChange}
            onMouseUp={handleSeekCommit}
            onPointerUp={handleSeekCommit}
          />
          <div className={styles.timeRow}>
            <span className={`label-sm ${styles.time}`}>{formatTime(currentTime)}</span>
            <span className={`label-sm ${styles.time}`}>{formatTime(duration)}</span>
          </div>
        </div>

        {/* Transport controls */}
        <div className={styles.transport}>
          <button
            className={`btn btn-icon ${styles.transportBtn}`}
            onClick={() => goToTrack(trackIdx - 1)}
            disabled={trackIdx === 0}
            aria-label="Previous track"
          >
            <RiSkipBackFill size={24} />
          </button>

          <button
            className={`btn btn-primary ${styles.playPauseBtn}`}
            onClick={togglePlay}
            aria-label={playing ? 'Pause' : 'Play'}
          >
            {playing ? <RiPauseFill size={28} /> : <RiPlayFill size={28} />}
          </button>

          <button
            className={`btn btn-icon ${styles.transportBtn}`}
            onClick={() => goToTrack(trackIdx + 1)}
            disabled={trackIdx === tracks.length - 1}
            aria-label="Next track"
          >
            <RiSkipForwardFill size={24} />
          </button>
        </div>

        {/* Secondary controls */}
        <div className={styles.secondary}>
          <div className={styles.volumeGroup}>
            <button className={`btn btn-icon ${styles.secondaryBtn}`} onClick={toggleMute} aria-label="Mute">
              <VolumeIcon size={18} />
            </button>
            <input
              type="range"
              min={0} max={1} step={0.02}
              value={muted ? 0 : volume}
              className={`${styles.seekBar} ${styles.volumeBar}`}
              style={{ '--progress': `${(muted ? 0 : volume) * 100}%` } as React.CSSProperties}
              onChange={handleVolumeChange}
            />
          </div>

          <button
            className={`btn ${styles.speedBtn}`}
            onClick={cycleSpeed}
            aria-label="Playback speed"
            title="Playback speed"
          >
            {SPEEDS[speedIdx]}×
          </button>

          {castReady && <CastButton className={styles.castBtn} />}

          <span className={`label-sm ${styles.trackCounter}`}>
            {trackIdx + 1} / {tracks.length}
          </span>
        </div>
      </main>

      {/* Queue panel */}
      {queueOpen && (
        <div className={styles.queuePanel}>
          <div className={styles.queuePanelHeader}>
            <h2 className={`headline-sm ${styles.queuePanelTitle}`}>Tracks</h2>
            <button className={`btn btn-icon ${styles.topBtn}`} onClick={() => setQueueOpen(false)}>
              <RiCloseLine size={20} />
            </button>
          </div>
          <div className={styles.queuePanelList}>
            {tracks.map((t, i) => (
              <button
                key={t.id}
                className={`${styles.queueItem} ${i === trackIdx ? styles.queueItemActive : ''}`}
                onClick={() => goToTrack(i)}
              >
                <span className={`label-sm ${styles.queueItemNum}`}>{t.number}</span>
                <span className={`label-md ${styles.queueItemTitle}`}>{t.title}</span>
                {t.duration > 0 && (
                  <span className={`label-sm ${styles.queueItemDuration}`}>{formatTime(t.duration)}</span>
                )}
                {i === trackIdx && <RiCheckLine size={16} className={styles.queueItemCheck} />}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function formatTime(s: number): string {
  if (!isFinite(s) || s < 0) return '0:00'
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = Math.floor(s % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
  return `${m}:${String(sec).padStart(2, '0')}`
}
