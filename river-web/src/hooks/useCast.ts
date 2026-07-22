import { useCallback, useEffect, useRef, useState } from 'react'

export function useCast() {
  const [castReady, setCastReady] = useState(false)
  const [isCasting, setIsCasting] = useState(false)
  const initializedRef = useRef(false)

  useEffect(() => {
    const init = () => {
      if (initializedRef.current) return
      initializedRef.current = true

      cast.framework.CastContext.getInstance().setOptions({
        receiverApplicationId: chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID,
        autoJoinPolicy: chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED,
      })

      const ctx = cast.framework.CastContext.getInstance()
      const onSessionChange = () => {
        const state = ctx.getSessionState()
        setIsCasting(
          state === cast.framework.SessionState.SESSION_STARTED ||
          state === cast.framework.SessionState.SESSION_RESUMED,
        )
      }
      ctx.addEventListener(cast.framework.CastContextEventType.SESSION_STATE_CHANGED, onSessionChange)
      setCastReady(true)
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    if ((window as any).cast?.framework) {
      init()
    } else {
      const prev = window.__onGCastApiAvailable
      window.__onGCastApiAvailable = (isAvailable: boolean) => {
        prev?.(isAvailable)
        if (isAvailable) init()
      }
    }
  }, [])

  const loadCastMedia = useCallback((
    contentUrl: string,
    contentType: string,
    metadata: unknown,
    currentTime?: number,
  ) => {
    const ctx = cast.framework.CastContext.getInstance()
    const session = ctx.getCurrentSession()
    if (!session) return
    const mediaInfo = new chrome.cast.media.MediaInfo(contentUrl, contentType)
    mediaInfo.metadata = metadata
    const request = new chrome.cast.media.LoadRequest(mediaInfo)
    if (currentTime != null && currentTime > 0) request.currentTime = currentTime
    session.loadMedia(request).catch(() => {})
  }, [])

  return { castReady, isCasting, loadCastMedia }
}
