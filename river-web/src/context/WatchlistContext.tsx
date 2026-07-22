import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { api } from '../api'
import type { WatchlistItem } from '../api'

interface WatchlistContextValue {
  items: WatchlistItem[]
  isInWatchlist: (mediaType: string, mediaId: string) => boolean
  findItem: (mediaType: string, mediaId: string) => WatchlistItem | undefined
  toggle: (mediaType: WatchlistItem['media_type'], mediaId: string) => void
}

const WatchlistContext = createContext<WatchlistContextValue | null>(null)

export function WatchlistProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<WatchlistItem[]>([])

  useEffect(() => {
    api.getWatchlist().then(setItems).catch(() => {})
  }, [])

  const findItem = useCallback(
    (mediaType: string, mediaId: string) =>
      items.find(i => i.media_type === mediaType && i.media_id === mediaId),
    [items],
  )

  const isInWatchlist = useCallback(
    (mediaType: string, mediaId: string) => !!findItem(mediaType, mediaId),
    [findItem],
  )

  const toggle = useCallback(
    (mediaType: WatchlistItem['media_type'], mediaId: string) => {
      const existing = items.find(i => i.media_type === mediaType && i.media_id === mediaId)
      if (existing) {
        // Optimistic remove
        setItems(prev => prev.filter(i => i.id !== existing.id))
        api.removeFromWatchlist(existing.id).catch(() => {
          setItems(prev => [...prev, existing])
        })
      } else {
        // Optimistic add — placeholder until server responds
        const placeholder: WatchlistItem = {
          id: `pending-${mediaType}-${mediaId}`,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          media_type: mediaType,
          media_id: mediaId,
          title: '',
          year: 0,
          poster_path: '',
          added_at: new Date().toISOString(),
        }
        setItems(prev => [placeholder, ...prev])
        api.addToWatchlist(mediaType, mediaId)
          .then(real => setItems(prev => prev.map(i => i.id === placeholder.id ? real : i)))
          .catch(() => setItems(prev => prev.filter(i => i.id !== placeholder.id)))
      }
    },
    [items],
  )

  return (
    <WatchlistContext.Provider value={{ items, isInWatchlist, findItem, toggle }}>
      {children}
    </WatchlistContext.Provider>
  )
}

export function useWatchlist() {
  const ctx = useContext(WatchlistContext)
  if (!ctx) throw new Error('useWatchlist must be used inside WatchlistProvider')
  return ctx
}
