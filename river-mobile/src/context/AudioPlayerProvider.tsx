import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { api } from '../api'
import { shouldResume, clampSeek } from '../util/player'
import { stepIndex, type AudioItem } from '../util/audio'
import { imageUrl } from '../util/imageUrl'
import { AudioPlayerContext, type AudioPlayerState } from './audioPlayerContext'
import { FullPlayer } from '../components/FullPlayer'

const PROGRESS_SEND_INTERVAL_S = 5
// Skip-back near the start of a track goes to the previous item instead.
const PREV_RESTART_THRESHOLD_S = 3
const MEDIA_ACTIONS = ['play', 'pause', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'seekto'] as const

/*
 * Global audio player. One long-lived <audio> element streams music tracks and
 * audiobook chapters and survives route changes, so the docked mini-player (and
 * its expanded full view) keep playing while the user browses.
 *
 * Owns: the queue, play/pause/seek/skip/next/prev, the Media Session metadata +
 * transport handlers (so the OS shows lock-screen / notification controls —
 * the groundwork for native background audio, #146), and progress resume +
 * throttled reporting for audiobook chapters (music tracks have no server-side
 * progress).
 *
 * Mounted inside the authenticated app only, so the progress socket never opens
 * on the login screen.
 */
export function AudioPlayerProvider({ children }: { children: ReactNode }) {
  const audioRef = useRef<HTMLAudioElement>(null)

  const [queue, setQueue] = useState<AudioItem[]>([])
  const [index, setIndex] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [expanded, setExpanded] = useState(false)

  const current = queue[index] ?? null

  // Refs so the interval / media-session / audio-event handlers read live
  // values without being re-registered on every state change. Synced in an
  // effect (not during render) per the rules-of-refs.
  const queueRef = useRef(queue)
  const indexRef = useRef(index)
  const currentRef = useRef(current)
  useEffect(() => {
    queueRef.current = queue
    indexRef.current = index
    currentRef.current = current
  })

  const playQueue = useCallback((items: AudioItem[], startIndex: number) => {
    setQueue(items)
    setIndex(Math.max(0, Math.min(startIndex, items.length - 1)))
  }, [])

  const jumpTo = useCallback((i: number) => {
    if (i >= 0 && i < queueRef.current.length) setIndex(i)
  }, [])

  const next = useCallback(() => {
    const n = stepIndex(indexRef.current, 1, queueRef.current.length)
    if (n != null) setIndex(n)
  }, [])

  const prev = useCallback(() => {
    const a = audioRef.current
    if (a && a.currentTime > PREV_RESTART_THRESHOLD_S) { a.currentTime = 0; return }
    const p = stepIndex(indexRef.current, -1, queueRef.current.length)
    if (p != null) setIndex(p)
  }, [])

  const toggle = useCallback(() => {
    const a = audioRef.current
    if (!a) return
    if (a.paused) void a.play().catch(() => {})
    else a.pause()
  }, [])

  const seek = useCallback((seconds: number) => {
    const a = audioRef.current
    if (a) a.currentTime = seconds
  }, [])

  const skip = useCallback((deltaSeconds: number) => {
    const a = audioRef.current
    if (a) a.currentTime = clampSeek(a.currentTime, deltaSeconds, a.duration || 0)
  }, [])

  const expand = useCallback(() => setExpanded(true), [])
  const collapse = useCallback(() => setExpanded(false), [])

  const close = useCallback(() => {
    const a = audioRef.current
    if (a) { a.pause(); a.removeAttribute('src'); a.load() }
    setQueue([]); setIndex(0); setExpanded(false)
    setPlaying(false); setPosition(0); setDuration(0)
    if ('mediaSession' in navigator) navigator.mediaSession.metadata = null
  }, [])

  // End of a track: advance to the next queued item, or close if it was last.
  const handleEnded = useCallback(() => {
    const n = stepIndex(indexRef.current, 1, queueRef.current.length)
    if (n != null) setIndex(n)
    else close()
  }, [close])

  // Load + play whenever the current item changes; resume chapters from saved
  // position. Sets .src imperatively so React doesn't re-trigger a load.
  useEffect(() => {
    const a = audioRef.current
    if (!a || !current) return
    a.src = current.streamUrl
    a.load()
    let cancelled = false
    const start = () => { if (!cancelled) void a.play().catch(() => {}) }
    if (current.progressKind) {
      api.getProgress(current.progressKind, current.id).then(p => {
        if (cancelled) return
        const apply = () => {
          if (p && shouldResume(p.position, p.duration)) a.currentTime = p.position
          start()
        }
        if (a.readyState >= 1) apply()
        else a.addEventListener('loadedmetadata', apply, { once: true })
      }).catch(start)
    } else {
      start()
    }
    return () => { cancelled = true }
    // Only the identity of the item matters; the rest is read live.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id])

  // Throttled progress reporting for chapters + a final flush on unmount.
  useEffect(() => {
    const sock = api.openProgressSocket()
    let lastSent = -PROGRESS_SEND_INTERVAL_S
    const iv = window.setInterval(() => {
      const a = audioRef.current, cur = currentRef.current
      if (!a || a.paused || !a.duration || !cur?.progressKind) return
      if (Math.abs(a.currentTime - lastSent) >= PROGRESS_SEND_INTERVAL_S) {
        sock.send(cur.progressKind, cur.id, a.currentTime, a.duration)
        lastSent = a.currentTime
      }
    }, 1000)
    return () => {
      // eslint-disable-next-line react-hooks/exhaustive-deps -- read the live ref at unmount to flush the final chapter position
      const a = audioRef.current, cur = currentRef.current
      if (a && a.duration && cur?.progressKind) sock.send(cur.progressKind, cur.id, a.currentTime, a.duration)
      window.clearInterval(iv)
      sock.close()
    }
  }, [])

  // Media Session transport handlers (registered once; call the stable methods).
  useEffect(() => {
    if (!('mediaSession' in navigator)) return
    const ms = navigator.mediaSession
    ms.setActionHandler('play', () => { void audioRef.current?.play().catch(() => {}) })
    ms.setActionHandler('pause', () => audioRef.current?.pause())
    ms.setActionHandler('previoustrack', () => prev())
    ms.setActionHandler('nexttrack', () => next())
    ms.setActionHandler('seekbackward', d => skip(-(d.seekOffset ?? 15)))
    ms.setActionHandler('seekforward', d => skip(d.seekOffset ?? 15))
    ms.setActionHandler('seekto', d => { if (d.seekTime != null) seek(d.seekTime) })
    return () => { for (const action of MEDIA_ACTIONS) ms.setActionHandler(action, null) }
  }, [prev, next, skip, seek])

  // Media Session metadata follows the current item.
  useEffect(() => {
    if (!('mediaSession' in navigator) || !current) return
    const art = imageUrl(current.artworkPath, 'poster')
    navigator.mediaSession.metadata = new MediaMetadata({
      title: current.title,
      artist: current.artist,
      album: current.album,
      artwork: art ? [{ src: art, sizes: '512x512', type: 'image/jpeg' }] : [],
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id])

  const value: AudioPlayerState = useMemo(() => ({
    current, queue, index, playing, position, duration, expanded,
    playQueue, toggle, seek, skip, next, prev, jumpTo, expand, collapse, close,
  }), [current, queue, index, playing, position, duration, expanded,
    playQueue, toggle, seek, skip, next, prev, jumpTo, expand, collapse, close])

  return (
    <AudioPlayerContext.Provider value={value}>
      {children}
      <audio
        ref={audioRef}
        preload="auto"
        onPlay={() => { setPlaying(true); if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'playing' }}
        onPause={() => { setPlaying(false); if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'paused' }}
        onTimeUpdate={e => {
          const a = e.currentTarget
          setPosition(a.currentTime)
          if ('mediaSession' in navigator && a.duration && navigator.mediaSession.setPositionState) {
            try {
              navigator.mediaSession.setPositionState({ duration: a.duration, position: a.currentTime, playbackRate: a.playbackRate })
            } catch { /* ignore transiently-invalid position state */ }
          }
        }}
        onDurationChange={e => setDuration(e.currentTarget.duration)}
        onEnded={handleEnded}
      />
      {expanded && current && <FullPlayer />}
    </AudioPlayerContext.Provider>
  )
}
