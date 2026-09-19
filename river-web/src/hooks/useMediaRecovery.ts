import { useCallback, useEffect, useRef, type RefObject } from 'react'
import { api } from '../api'

// Don't attempt recovery more than once within this window, so a genuinely
// dead stream (which re-fires `error` on every reload) can't spin.
const RECOVERY_COOLDOWN_MS = 8000

/**
 * Recovers a stalled <video>/<audio> element after a long pause / device
 * sleep.
 *
 * While media is paused, the browser/OS tears down the stream's underlying
 * HTTP Range connection. The element keeps its buffered frames but has no
 * live connection, so a later play() "succeeds" (fires `playing`) yet the
 * timeline never advances — the player looks like it's resuming but is
 * frozen. If the pause outlasts the stream token (~8h), any reconnection
 * also 401s.
 *
 * This hook reloads the source in place at the saved position — refreshing
 * the stream token first so the fresh URL is valid even after >8h — so
 * playback continues without the user having to leave the page. It is
 * self-contained: it captures the current position/play-state and restores
 * them itself on reload, so it works for any media element regardless of
 * how the page manages its own seek/resume state.
 *
 * Wire the returned `onError` into the element's onError, and call
 * `recover()` from a togglePlay guard when a play() doesn't advance the
 * timeline. A visibility listener also recovers pre-emptively when a
 * backgrounded tab / slept device returns and playback is stuck.
 */
export function useMediaRecovery(
  mediaRef: RefObject<HTMLMediaElement | null>,
  buildSrc: () => string | undefined,
  setSrc: (src: string) => void,
) {
  const lastRecoverRef = useRef(0)
  // True while a reload is in flight. Reloading the src resets the element's
  // currentTime to 0 and re-fires `timeupdate`, so callers consult this to
  // hold their displayed-time state until the saved position is restored —
  // otherwise the timer visibly flashes to 0:00 during the reload.
  const recoveringRef = useRef(false)

  const recover = useCallback(async () => {
    const el = mediaRef.current
    if (!el) return
    // Already reloading, or reloaded recently: don't stack reloads. The
    // cooldown also stops a genuinely dead stream (which re-fires `error` on
    // every reload) from spinning.
    if (recoveringRef.current) return
    const now = Date.now()
    if (now - lastRecoverRef.current < RECOVERY_COOLDOWN_MS) return
    lastRecoverRef.current = now

    const resumeAt = el.currentTime
    const wasPlaying = !el.paused
    recoveringRef.current = true

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
    if (!src) {
      recoveringRef.current = false
      return
    }

    // Safety net: if the reload never reaches `loadedmetadata` (e.g. a truly
    // dead stream), clear the flag anyway so the displayed time isn't frozen
    // forever.
    const clearGuard = window.setTimeout(() => {
      recoveringRef.current = false
    }, RECOVERY_COOLDOWN_MS)

    // Restore position + play-state once the reloaded element has metadata.
    // React reuses the same DOM node (only the src attribute changes), so
    // this fires on the element we just told to reload.
    el.addEventListener(
      'loadedmetadata',
      () => {
        window.clearTimeout(clearGuard)
        const m = mediaRef.current
        if (!m) {
          recoveringRef.current = false
          return
        }
        if (resumeAt > 0) m.currentTime = resumeAt
        if (wasPlaying) m.play().catch(() => {})
        // Position restored — let displayed time track the element again.
        recoveringRef.current = false
      },
      { once: true },
    )

    // A cache-busting param guarantees the src string changes even when the
    // token is unchanged, so React re-sets it and the element reloads.
    setSrc(`${src}${src.includes('?') ? '&' : '?'}_r=${Date.now()}`)
  }, [mediaRef, buildSrc, setSrc])

  // Surface a media error as a reload — but only for errors that mean the
  // stream is actually dead. MEDIA_ERR_ABORTED fires from our own src
  // reassignment (the reload above) and from navigating away; treating it as
  // a fault would reload spuriously or spin.
  const onError = useCallback(() => {
    if (mediaRef.current?.error?.code === MediaError.MEDIA_ERR_ABORTED) return
    void recover()
  }, [mediaRef, recover])

  // Returning to a backgrounded tab (or waking the device) is the moment
  // the frozen state is noticed. If the element thinks it's playing but the
  // timeline doesn't move over a short window, reload.
  useEffect(() => {
    function onVisible() {
      if (document.visibilityState !== 'visible') return
      const el = mediaRef.current
      if (!el || el.paused) return
      const before = el.currentTime
      window.setTimeout(() => {
        const m = mediaRef.current
        if (m && !m.paused && m.currentTime === before) void recover()
      }, 1500)
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [mediaRef, recover])

  return { recover, onError, recoveringRef }
}
