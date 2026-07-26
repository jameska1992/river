import { useCallback, useEffect, useRef, type RefObject } from 'react'
import { api } from '../api'

// Don't attempt recovery more than once within this window, so a genuinely
// dead stream (which re-fires `error` on every reload) can't spin.
const RECOVERY_COOLDOWN_MS = 8000

// How long to wait after a resume / tab-return before deciding the timeline
// is frozen (playing state but currentTime not advancing).
const FREEZE_GRACE_MS = 1500

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
 * playback continues without the user having to leave the player. It is
 * self-contained: it captures the current position/play-state and restores
 * them itself on reload, so it works for any media element regardless of
 * how the caller manages its own seek/resume state.
 *
 * Wire the returned `onError` into the element's onError, and call
 * `recover()` from a togglePlay guard when a play() doesn't advance the
 * timeline. A visibility listener also recovers pre-emptively when a
 * backgrounded app / slept device returns and playback is stuck — the
 * common case on a TV.
 */
export function useMediaRecovery(
  mediaRef: RefObject<HTMLMediaElement | null>,
  buildSrc: () => string | undefined,
  setSrc: (src: string) => void,
) {
  const lastRecoverRef = useRef(0)

  const recover = useCallback(async () => {
    const el = mediaRef.current
    if (!el) return
    const now = Date.now()
    if (now - lastRecoverRef.current < RECOVERY_COOLDOWN_MS) return
    lastRecoverRef.current = now

    const resumeAt = el.currentTime
    const wasPlaying = !el.paused

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

    // Restore position + play-state once the reloaded element has metadata.
    // React reuses the same DOM node (only the src attribute changes), so
    // this fires on the element we just told to reload.
    el.addEventListener(
      'loadedmetadata',
      () => {
        const m = mediaRef.current
        if (!m) return
        if (resumeAt > 0) m.currentTime = resumeAt
        if (wasPlaying) m.play().catch(() => {})
      },
      { once: true },
    )

    // A cache-busting param guarantees the src string changes even when the
    // token is unchanged, so React re-sets it and the element reloads.
    setSrc(`${src}${src.includes('?') ? '&' : '?'}_r=${Date.now()}`)
  }, [mediaRef, buildSrc, setSrc])

  // Surface a fatal media error (e.g. the stream token 401'd) as a reload.
  const onError = useCallback(() => {
    void recover()
  }, [recover])

  // Returning to a backgrounded app (or waking the device) is the moment
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
      }, FREEZE_GRACE_MS)
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [mediaRef, recover])

  return { recover, onError, freezeGraceMs: FREEZE_GRACE_MS }
}
