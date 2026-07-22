import { useEffect, useRef, useState, useCallback } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import {
  RiArrowLeftLine,
  RiPlayFill, RiPauseFill,
  RiSkipBackFill, RiSkipForwardFill,
  RiVolumeUpLine, RiVolumeDownLine, RiVolumeMuteLine,
  RiListUnordered, RiCloseLine, RiCheckLine,
  RiHeadphoneLine,
} from 'react-icons/ri'
import { useAudiobooks } from '../context/AudiobooksContext'
import type { Audiobook, AudiobookChapter } from '../api'
import { api } from '../api'
import { useCast } from '../hooks/useCast'
import { CastButton } from '../components/CastButton'
import { imageUrl } from '../util/imageUrl'
import styles from './AudiobookListenPage.module.css'

const SPEEDS = [0.75, 1, 1.25, 1.5, 1.75, 2]
const SKIP_SECONDS = 30

export function AudiobookListenPage() {
  const { audiobookId } = useParams<{ audiobookId: string }>()
  const [searchParams] = useSearchParams()
  const startChapterId = searchParams.get('chapter')
  const { getOne, fetchChapters, chapterStreamUrl } = useAudiobooks()

  const audioRef = useRef<HTMLAudioElement>(null)
  const lastReportRef = useRef(0)
  // progressRef carries the latest (position, duration) into the unmount
  // cleanup so we can flush a final report even when the user closes the
  // tab mid-chapter — without it the WebSocket would close before the
  // next throttled onTimeUpdate had a chance to send.
  const progressRef = useRef<{ position: number; duration: number; chapterId: string | null }>({
    position: 0,
    duration: 0,
    chapterId: null,
  })
  const wsRef = useRef<ReturnType<typeof api.openProgressSocket> | null>(null)
  // pendingSeekRef holds a position to seek to once the new chapter's
  // metadata has loaded — used both for resuming a saved chapter from
  // the server and for re-entering a chapter via the URL ?chapter param.
  const pendingSeekRef = useRef<number | null>(null)

  const [book, setBook] = useState<Audiobook | null>(null)
  const [chapters, setChapters] = useState<AudiobookChapter[]>([])
  const [chapterIdx, setChapterIdx] = useState(0)
  const [isLoading, setIsLoading] = useState(true)

  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [seeking, setSeeking] = useState(false)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)
  const [speedIdx, setSpeedIdx] = useState(1) // 1x default
  const [chaptersOpen, setChaptersOpen] = useState(false)

  useEffect(() => {
    if (!audiobookId) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resets loading state before refetching when the audiobook id changes
    setIsLoading(true)
    Promise.all([getOne(audiobookId), fetchChapters(audiobookId)])
      .then(([b, chs]) => {
        setBook(b)
        const sorted = [...chs].sort((a, b) => a.number - b.number)
        setChapters(sorted)
        if (startChapterId) {
          const idx = sorted.findIndex(c => c.id === startChapterId)
          if (idx >= 0) setChapterIdx(idx)
        }
      })
      .finally(() => setIsLoading(false))
  }, [audiobookId, getOne, fetchChapters, startChapterId])

  const currentChapter = chapters[chapterIdx] ?? null

  // pendingPlayRef is a "should the new chapter auto-start once it loads"
  // signal, set by whoever caused the chapter change (auto-advance from
  // onEnded, the next/prev buttons, the chapter list). The chapter-change
  // effect below *must not* touch this ref — onEnded sets it to true after
  // the natural pause event has flipped `playing` to false, and any reset
  // here would silently undo that. onLoadedMetadata consumes the ref.
  const pendingPlayRef = useRef(false)
  const [audioSrc, setAudioSrc] = useState<string | undefined>()

  useEffect(() => {
    if (!audiobookId || !currentChapter) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resets playback position when the current chapter changes
    setCurrentTime(0)
    lastReportRef.current = 0
    setAudioSrc(chapterStreamUrl(audiobookId, currentChapter.id))
    progressRef.current = { position: 0, duration: 0, chapterId: currentChapter.id }

    // Resume the user's saved position for this specific chapter. We seek
    // once metadata has loaded so the audio element has a valid duration
    // and currentTime is honored; if metadata is already there (rare,
    // browser may cache), set immediately.
    api.getProgress('chapter', currentChapter.id).then(p => {
      if (!p || p.completed) return
      if (p.position > 5 && (p.duration <= 0 || p.position < p.duration - 30)) {
        const a = audioRef.current
        if (a && a.readyState >= 1) {
          a.currentTime = p.position
        } else {
          pendingSeekRef.current = p.position
        }
      }
    }).catch(() => { /* 404 = no progress yet, fine */ })
  }, [audiobookId, currentChapter?.id]) // eslint-disable-line react-hooks/exhaustive-deps

  // One progress socket for the lifetime of this page — chapters share it.
  // Closes on unmount, with a final flush so a half-second-stale position
  // doesn't get lost when the tab is closed during playback.
  useEffect(() => {
    wsRef.current = api.openProgressSocket()
    return () => {
      const { position, duration, chapterId } = progressRef.current
      if (chapterId && position > 5) {
        wsRef.current?.send('chapter', chapterId, position, duration)
      }
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [])

  // Apply speed to audio element whenever it changes
  useEffect(() => {
    const a = audioRef.current
    if (!a) return
    a.playbackRate = SPEEDS[speedIdx]
  }, [speedIdx])

  const goToChapter = useCallback((idx: number) => {
    if (idx < 0 || idx >= chapters.length) return
    // Preserve the user's current play/pause state across the chapter
    // switch — if they were listening, the new chapter resumes playback;
    // if they were paused (e.g. just scrolling through chapters), it
    // stays paused. Reading audioRef directly avoids capturing stale
    // `playing` state through the useCallback closure.
    pendingPlayRef.current = !audioRef.current?.paused
    setChapterIdx(idx)
    setChaptersOpen(false)
  }, [chapters.length])

  const { castReady, isCasting, loadCastMedia } = useCast()

  useEffect(() => {
    if (!isCasting || !book || !currentChapter || !audioSrc) return
    const meta = new chrome.cast.media.GenericMediaMetadata()
    meta.title = currentChapter.title
      ? `${book.title} · ${currentChapter.title}`
      : `${book.title} · Chapter ${currentChapter.number}`
    if (book.cover_path) meta.images = [{ url: book.cover_path }]
    loadCastMedia(audioSrc, 'audio/mp4', meta, audioRef.current?.currentTime)
  }, [isCasting, book, currentChapter, audioSrc, loadCastMedia])

  const onLoadedMetadata = () => {
    const a = audioRef.current
    if (!a) return
    setDuration(a.duration)
    a.playbackRate = SPEEDS[speedIdx]
    // Consume any saved-position seek queued by the per-chapter progress
    // fetch above. Must happen before play() so we don't briefly play
    // from 0 before jumping.
    if (pendingSeekRef.current !== null) {
      a.currentTime = pendingSeekRef.current
      pendingSeekRef.current = null
    }
    if (pendingPlayRef.current) {
      pendingPlayRef.current = false
      a.play().catch(() => {})
    }
  }

  const onTimeUpdate = () => {
    const a = audioRef.current
    if (!a || seeking) return
    setCurrentTime(a.currentTime)
    // Keep the unmount-flush ref hot.
    progressRef.current = {
      position: a.currentTime,
      duration: a.duration || 0,
      chapterId: currentChapter?.id ?? null,
    }
    // Throttle wire reports to once every ~10s. The video pages use the
    // same cadence — it's enough fidelity for continue-watching without
    // hammering the WS connection. lastReportRef carries the last-sent
    // position so the next tick can compare.
    if (currentChapter && Math.abs(a.currentTime - lastReportRef.current) >= 10) {
      lastReportRef.current = a.currentTime
      wsRef.current?.send('chapter', currentChapter.id, a.currentTime, a.duration || 0)
    }
  }

  const onEnded = () => {
    // Auto-advance to next chapter and keep playing. Don't call
    // setPlaying(false) here — the browser already fired `pause` right
    // before `ended` and onPause flipped the state. pendingPlayRef=true
    // tells onLoadedMetadata to resume playback once the next chapter's
    // audio is loaded. If this is the final chapter, fall through and let
    // the natural pause state stand.
    if (chapterIdx < chapters.length - 1) {
      pendingPlayRef.current = true
      setChapterIdx(i => i + 1)
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

  const skip = useCallback((secs: number) => {
    const a = audioRef.current
    if (!a) return
    a.currentTime = Math.max(0, Math.min(a.duration, a.currentTime + secs))
  }, [])

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
        case 'ArrowLeft': skip(-SKIP_SECONDS); break
        case 'ArrowRight': skip(SKIP_SECONDS); break
        case 'n': case 'N': goToChapter(chapterIdx + 1); break
        case 'p': case 'P': goToChapter(chapterIdx - 1); break
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [togglePlay, toggleMute, skip, goToChapter, chapterIdx])

  const progress = duration > 0 ? currentTime / duration : 0
  const VolumeIcon = muted || volume === 0 ? RiVolumeMuteLine : volume < 0.5 ? RiVolumeDownLine : RiVolumeUpLine

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.backdrop} />
        <div className={styles.loadingState}>
          <RiHeadphoneLine size={48} style={{ color: 'rgba(255,255,255,0.3)' }} />
        </div>
      </div>
    )
  }

  if (!book || !currentChapter) return null

  return (
    <div className={styles.page}>
      {/* Blurred cover backdrop */}
      {book.cover_path && (
        <div
          className={styles.backdrop}
          style={{ backgroundImage: `url(${book.cover_path})` }}
        />
      )}
      <div className={styles.backdropOverlay} />

      {/* Hidden audio element */}
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
        <Link to={`/audiobook/${audiobookId}`} className={`btn btn-icon ${styles.topBtn}`}>
          <RiArrowLeftLine size={20} />
        </Link>
        <span className={`label-md ${styles.topTitle}`}>
          {book.title}
        </span>
        <button
          className={`btn btn-icon ${styles.topBtn} ${chaptersOpen ? styles.topBtnActive : ''}`}
          onClick={() => setChaptersOpen(o => !o)}
          aria-label="Chapters"
          title="Chapters"
        >
          <RiListUnordered size={20} />
        </button>
      </div>

      {/* Main player */}
      <main className={styles.player}>
        {/* Cover art */}
        <div className={styles.coverWrap}>
          {book.cover_path ? (
            <img src={imageUrl(book.cover_path)} alt={book.title} className={styles.cover} />
          ) : (
            <div className={styles.coverFallback}>
              <RiHeadphoneLine size={64} />
            </div>
          )}
        </div>

        {/* Track info */}
        <div className={styles.info}>
          <p className={`headline-sm ${styles.infoTitle}`}>{book.title}</p>
          {book.author && <p className={`label-md ${styles.infoAuthor}`}>{book.author}</p>}
          <p className={`label-sm ${styles.infoChapter}`}>
            Chapter {currentChapter.number}
            {currentChapter.title ? ` · ${currentChapter.title}` : ''}
          </p>
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
            onClick={() => goToChapter(chapterIdx - 1)}
            disabled={chapterIdx === 0}
            aria-label="Previous chapter"
          >
            <RiSkipBackFill size={24} />
          </button>

          <button
            className={`btn btn-icon ${styles.transportBtn}`}
            onClick={() => skip(-SKIP_SECONDS)}
            aria-label={`Skip back ${SKIP_SECONDS}s`}
          >
            <span className={styles.skipLabel}>–{SKIP_SECONDS}</span>
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
            onClick={() => skip(SKIP_SECONDS)}
            aria-label={`Skip forward ${SKIP_SECONDS}s`}
          >
            <span className={styles.skipLabel}>+{SKIP_SECONDS}</span>
          </button>

          <button
            className={`btn btn-icon ${styles.transportBtn}`}
            onClick={() => goToChapter(chapterIdx + 1)}
            disabled={chapterIdx === chapters.length - 1}
            aria-label="Next chapter"
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

          <span className={`label-sm ${styles.chapterCounter}`}>
            {chapterIdx + 1} / {chapters.length}
          </span>
        </div>
      </main>

      {/* Chapters panel */}
      {chaptersOpen && (
        <div className={styles.chaptersPanel}>
          <div className={styles.chaptersPanelHeader}>
            <h2 className={`headline-sm ${styles.chaptersPanelTitle}`}>Chapters</h2>
            <button className={`btn btn-icon ${styles.topBtn}`} onClick={() => setChaptersOpen(false)}>
              <RiCloseLine size={20} />
            </button>
          </div>
          <div className={styles.chaptersPanelList}>
            {chapters.map((ch, i) => (
              <button
                key={ch.id}
                className={`${styles.chapterItem} ${i === chapterIdx ? styles.chapterItemActive : ''}`}
                onClick={() => goToChapter(i)}
              >
                <span className={`label-sm ${styles.chapterItemNum}`}>{ch.number}</span>
                <span className={`label-md ${styles.chapterItemTitle}`}>
                  {ch.title || `Chapter ${ch.number}`}
                </span>
                {ch.duration > 0 && (
                  <span className={`label-sm ${styles.chapterItemDuration}`}>{formatDuration(ch.duration)}</span>
                )}
                {i === chapterIdx && <RiCheckLine size={16} className={styles.chapterItemCheck} />}
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

function formatDuration(secs: number): string {
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}
