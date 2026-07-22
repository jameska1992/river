import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type Collection } from '../api'
import { FocusProvider } from '../hooks/useFocus'
import { Sidebar } from '../components/Sidebar'
import { CollectionCard } from '../components/CollectionCard'

// Landscape art needs more pixels per card to keep the 2×2 collage
// readable, so 5 columns instead of the movies/shows grid's 10.
const COLS = 5

export default function CollectionsPage() {
  const navigate = useNavigate()
  const [collections, setCollections] = useState<Collection[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const mainRef = useRef<HTMLElement>(null)

  useEffect(() => {
    let alive = true
    api.listCollections()
      .then(c => { if (alive) setCollections(c) })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
      .finally(() => { if (alive) setLoaded(true) })
    return () => { alive = false }
  }, [])

  return (
    <FocusProvider onBack={() => navigate('/')}>
      <div style={styles.page}>
        <Sidebar />
        <main ref={mainRef} style={styles.main}>
          {error && <div style={styles.error}>{error}</div>}

          <div style={styles.heading}>
            <h2 style={styles.title}>Collections</h2>
            <span style={styles.count}>
              {collections.length > 0 && `${collections.length} ${collections.length === 1 ? 'collection' : 'collections'}`}
            </span>
          </div>

          {loaded && collections.length === 0 && !error && (
            <p style={styles.empty}>No collections yet.</p>
          )}

          <div style={styles.grid}>
            {collections.map((col, i) => {
              const row = Math.floor(i / COLS)
              const lastRow = Math.floor((collections.length - 1) / COLS)
              const onFocus =
                row === 0       ? () => mainRef.current?.scrollTo({ top: 0, behavior: 'smooth' }) :
                row === lastRow ? () => mainRef.current?.scrollTo({ top: mainRef.current.scrollHeight, behavior: 'smooth' }) :
                undefined
              return (
                <CollectionCard
                  key={col.id}
                  name={col.name}
                  itemCount={col.item_count}
                  covers={col.covers ?? []}
                  fill
                  autoFocus={i === 0}
                  overrides={{
                    left: (i % COLS) === 0 ? 'sidebar' : undefined,
                  }}
                  onFocus={onFocus}
                  onSelect={() => navigate(`/collections/${col.id}`)}
                />
              )
            })}
          </div>
        </main>
      </div>
    </FocusProvider>
  )
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    height: '100vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    paddingLeft: 'var(--sidebar-rail)',
  },
  main: {
    flex: 1,
    overflowY: 'auto',
    padding: 'calc(var(--safe-y) + 1.5rem) var(--safe-x) calc(var(--safe-y) + 1.5rem)',
    scrollbarWidth: 'none',
  },
  heading: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '1rem',
    marginBottom: '1.5rem',
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
    margin: '2rem 0',
  },
  error: {
    margin: '0 0 1.5rem',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}
