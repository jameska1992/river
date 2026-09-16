import { useState } from 'react'
import { RiCloseLine } from 'react-icons/ri'
import { api } from '../api'
import type { WatchlistItem } from '../api'
import { useAsync } from '../hooks/useAsync'
import { Grid } from '../components/collections'
import { PosterCard } from '../components/PosterCard'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { detailPath } from '../util/format'
import { heading, screen } from './styles'

export default function WatchlistPage() {
  const { data, loading, error, reload } = useAsync(() => api.getWatchlist(), [])
  const [items, setItems] = useState<WatchlistItem[] | null>(null)
  // Prefer the locally-mutated copy (after a remove) over the fetched one.
  const list = items ?? data

  const remove = async (item: WatchlistItem) => {
    const base = list ?? []
    setItems(base.filter(i => i.id !== item.id))
    try { await api.removeFromWatchlist(item.id) } catch { setItems(base) }
  }

  return (
    <div>
      <h1 style={{ ...heading, ...screen, paddingBottom: '0.75rem' }}>Watchlist</h1>

      {loading ? <Loading />
        : error ? <ErrorState message={error} onRetry={reload} />
        : !list || list.length === 0 ? <EmptyState message="Your watchlist is empty." />
        : (
          <Grid>
            {list.map(item => (
              <div key={item.id} style={{ position: 'relative' }}>
                <PosterCard
                  to={detailPath(item.media_type, item.media_id)}
                  title={item.title}
                  subtitle={item.year ? String(item.year) : undefined}
                  image={item.poster_path}
                  kind={item.media_type}
                />
                <button
                  onClick={() => void remove(item)}
                  aria-label={`Remove ${item.title} from watchlist`}
                  style={styles.remove}
                >
                  <RiCloseLine size={16} />
                </button>
              </div>
            ))}
          </Grid>
        )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  remove: {
    position: 'absolute', top: 6, right: 6,
    width: 28, height: 28, borderRadius: '50%',
    background: 'rgba(0,0,0,0.65)', color: '#fff',
    display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
}
