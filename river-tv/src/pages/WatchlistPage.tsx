import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchlistItem } from '../api'
import { FocusProvider } from '../hooks/useFocus'
import { Sidebar } from '../components/Sidebar'
import { PosterCard } from '../components/PosterCard'

const COLS = 10

export default function WatchlistPage() {
  const navigate = useNavigate()
  const [items, setItems] = useState<WatchlistItem[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const pageRef = useRef<HTMLDivElement>(null)

  const scrollToTop = () =>
    pageRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
  const scrollToBottom = () =>
    pageRef.current?.scrollTo({ top: pageRef.current.scrollHeight, behavior: 'smooth' })

  useEffect(() => {
    let alive = true
    api.getWatchlist()
      .then(list => {
        if (!alive) return
        // Server already returns newest-first. Just sort defensively
        // so we never display in random order if the contract changes.
        setItems([...list].sort((a, b) => b.added_at.localeCompare(a.added_at)))
      })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
      .finally(() => { if (alive) setLoaded(true) })
    return () => { alive = false }
  }, [])

  function open(item: WatchlistItem) {
    switch (item.media_type) {
      case 'movie':     navigate(`/movies/${item.media_id}`);     break
      case 'tvshow':    navigate(`/tvshows/${item.media_id}`);    break
      case 'audiobook': navigate(`/audiobooks/${item.media_id}`); break
    }
  }

  const lastRow = Math.floor((items.length - 1) / COLS)

  return (
    <FocusProvider onBack={() => navigate('/')}>
      <div ref={pageRef} style={styles.page}>
        <Sidebar />

        {error && <div style={styles.error}>{error}</div>}

        <main style={styles.main}>
          <div style={styles.heading}>
            <h2 style={styles.title}>Watchlist</h2>
            <span style={styles.count}>
              {items.length > 0 && `${items.length} ${items.length === 1 ? 'item' : 'items'}`}
            </span>
          </div>

          {loaded && items.length === 0 && !error && (
            <p style={styles.empty}>
              Your watchlist is empty. Add a movie, TV show or audiobook from its detail page.
            </p>
          )}

          {items.length > 0 && (
            <div style={styles.grid}>
              {items.map((it, i) => {
                const row = Math.floor(i / COLS)
                return (
                  <PosterCard
                    key={it.id}
                    title={it.title}
                    subtitle={subtitle(it)}
                    imageSrc={it.poster_path || undefined}
                    fill
                    autoFocus={i === 0}
                    overrides={{
                      left: (i % COLS) === 0 ? 'sidebar' : undefined,
                    }}
                    onSelect={() => open(it)}
                    onFocus={
                      row === 0       ? scrollToTop :
                      row === lastRow ? scrollToBottom :
                      undefined
                    }
                  />
                )
              })}
            </div>
          )}
        </main>
      </div>
    </FocusProvider>
  )
}

function subtitle(it: WatchlistItem): string | undefined {
  const kind =
    it.media_type === 'movie'     ? 'Movie' :
    it.media_type === 'tvshow'    ? 'Series' :
    it.media_type === 'audiobook' ? 'Audiobook' :
                                    null
  if (kind && it.year > 0) return `${kind} · ${it.year}`
  if (kind) return kind
  return it.year ? String(it.year) : undefined
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    height: '100vh',
    overflowY: 'auto',
    paddingLeft: 'var(--sidebar-rail)',
    scrollbarWidth: 'none',
  },
  main: {
    padding: 'calc(var(--safe-y) + 1.5rem) var(--safe-x) calc(var(--safe-y) + 1.5rem)',
    display: 'flex',
    flexDirection: 'column',
    gap: '1.5rem',
  },
  heading: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '1rem',
  },
  title: {
    margin: 0,
    fontSize: '1.75rem',
    fontWeight: 600,
  },
  count: {
    color: 'var(--text-muted)',
    fontSize: '1rem',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: `repeat(${COLS}, minmax(0, 1fr))`,
    gap: '1.5rem 1.25rem',
  },
  empty: {
    color: 'var(--text-muted)',
    fontSize: '1rem',
    maxWidth: '40rem',
  },
  error: {
    margin: 'var(--safe-y) var(--safe-x)',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}
