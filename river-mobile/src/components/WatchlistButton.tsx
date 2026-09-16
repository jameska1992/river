import { useEffect, useState } from 'react'
import { RiBookmarkLine, RiBookmarkFill } from 'react-icons/ri'
import { api } from '../api'

type WatchlistMedia = 'movie' | 'tvshow' | 'audiobook'

// Self-contained add/remove-from-watchlist toggle. Resolves current membership
// on mount (from the watchlist list, since there's no single-item check) and
// tracks the created item id so it can be removed.
export function WatchlistButton({ mediaType, mediaId }: { mediaType: WatchlistMedia; mediaId: string }) {
  const [itemId, setItemId] = useState<string | null>(null)
  const [ready, setReady] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let active = true
    api.getWatchlist()
      .then(list => { if (active) setItemId(list.find(i => i.media_type === mediaType && i.media_id === mediaId)?.id ?? null) })
      .catch(() => { /* treat as not-in-list */ })
      .finally(() => { if (active) setReady(true) })
    return () => { active = false }
  }, [mediaType, mediaId])

  const toggle = async () => {
    if (busy) return
    setBusy(true)
    try {
      if (itemId) {
        await api.removeFromWatchlist(itemId)
        setItemId(null)
      } else {
        const item = await api.addToWatchlist(mediaType, mediaId)
        setItemId(item.id)
      }
    } catch { /* leave state as-is on failure */ } finally {
      setBusy(false)
    }
  }

  const inList = !!itemId
  return (
    <button className="btn" onClick={() => void toggle()} disabled={!ready || busy} style={{ gap: '0.5rem' }}>
      {inList ? <RiBookmarkFill /> : <RiBookmarkLine />}
      {inList ? 'In watchlist' : 'Watchlist'}
    </button>
  )
}
