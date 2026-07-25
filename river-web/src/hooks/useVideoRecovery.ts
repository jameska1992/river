import { useCallback, useEffect, useRef, type RefObject } from 'react'
import { api } from '../api'

// Don't attempt recovery more than once within this window, so a genuinely
// dead stream (which re-fires `error` on every reload) can't spin.
const RECOVERY_COOLDOWN_MS = 8000

/**
 * Recovers a stalled <video> after a long pause / device sleep.
 *
 * While paused, the browser/OS tears down the video's underlying HTTP
 * Range connection to the stream. The element keeps its buffered frames
 * but has no live connection, so a later play() "succeeds" (fires
 * `playing`) yet the timeline never advances — the player looks like it's
 * resuming but is frozen. If the pause outlasts the stream token (~8h),
 * any reconnection also 401s.
 *
 * This hook reloads the source at the saved position — refreshing the
 * stream token first so the fresh URL is valid even after >8h — so
 * playback continues in place instead of the user having to leave the
 * page. It reuses the pages' existing pendingSeek/pendingPlay refs, which
 * their onLoadedMetadata handler already honours to seek + resume.
 */
export function useVideoRecovery(
  videoRef: RefObject<HTMLVideoElement | null>,
  buildSrc: () => string | undefined,
  setVideoSrc: (src: string) => void,
  pendingSeekRef: RefObject<number | null>,
  pendingPlayRef: RefObject<boolean>,
) {
  const lastRecoverRef = useRef(0)

  const recover = useCallback(async () => {
    const v = videoRef.current
    if (!v) return
    const now = Date.now()
    if (now - lastRecoverRef.current < RECOVERY_COOLDOWN_MS) return
    lastRecoverRef.current = now
    // Resume from where we were, and start playing once reloaded.
    pendingSeekRef.current = v.currentTime || pendingSeekRef.current || 0
    pendingPlayRef.current = true
    // A fresh stream token covers the case where the old one expired while
    // asleep. Proceed even if this fails — a still-valid token reload can
    // succeed on its own, and a genuinely dead session will 401 and route
    // to login on the next API call.
    try {
      await api.refreshStreamToken()
    } catch {
      /* ignore — attempt the reload regardless */
    }
    const src = buildSrc()
    if (!src) return
    // A cache-busting param guarantees the src string changes even when the
    // token is unchanged, so React re-sets it and the element reloads.
    setVideoSrc(`${src}${src.includes('?') ? '&' : '?'}_r=${Date.now()}`)
  }, [videoRef, buildSrc, setVideoSrc, pendingSeekRef, pendingPlayRef])

  // Surface a fatal media error (e.g. the stream token 401'd) as a reload.
  const onError = useCallback(() => {
    void recover()
  }, [recover])

  // Returning to a backgrounded tab (or waking the device) is the moment
  // the frozen state is noticed. If the element thinks it's playing but the
  // timeline doesn't move over a short window, reload.
  useEffect(() => {
    function onVisible() {
      if (document.visibilityState !== 'visible') return
      const v = videoRef.current
      if (!v || v.paused) return
      const before = v.currentTime
      window.setTimeout(() => {
        const el = videoRef.current
        if (el && !el.paused && el.currentTime === before) void recover()
      }, 1500)
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [videoRef, recover])

  return { recover, onError }
}
