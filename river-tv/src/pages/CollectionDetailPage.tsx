import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type CollectionDetail } from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'
import { Sidebar } from '../components/Sidebar'
import { PosterCard } from '../components/PosterCard'

const COLS = 10

export default function CollectionDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [collection, setCollection] = useState<CollectionDetail | null>(null)
  const [error, setError] = useState<string | null>(null)
  const pageRef = useRef<HTMLDivElement>(null)

  const scrollToTop = () =>
    pageRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
  const scrollToBottom = () =>
    pageRef.current?.scrollTo({ top: pageRef.current.scrollHeight, behavior: 'smooth' })

  useEffect(() => {
    if (!id) return
    let alive = true
    api.getCollection(id)
      .then(c => { if (alive) setCollection(c) })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
    return () => { alive = false }
  }, [id])

  const items = collection?.items ?? []
  const lastRow = Math.floor((items.length - 1) / COLS)

  return (
    <FocusProvider backFocusesTag="sidebar" onBack={() => navigate(-1)}>
      <div ref={pageRef} style={styles.page}>
        <Sidebar />

        {error && <div style={styles.error}>{error}</div>}

        {collection && (
          <main style={styles.main}>
            <BackButton onSelect={() => navigate(-1)} onFocus={scrollToTop} />

            <header style={styles.header}>
              <h1 style={styles.name}>{collection.name}</h1>
              {collection.description && (
                <p style={styles.description}>{collection.description}</p>
              )}
              <div style={styles.count}>
                {items.length} {items.length === 1 ? 'item' : 'items'}
              </div>
            </header>

            {items.length === 0 ? (
              <p style={styles.empty}>This collection is empty.</p>
            ) : (
              <div style={styles.grid}>
                {items.map((it, i) => {
                  const row = Math.floor(i / COLS)
                  return (
                    <PosterCard
                      key={it.id}
                      title={it.title}
                      subtitle={it.year ? String(it.year) : undefined}
                      imageSrc={it.poster_path || undefined}
                      fill
                      overrides={{
                        left: (i % COLS) === 0 ? 'sidebar' : undefined,
                      }}
                      onSelect={() => {
                        if (it.media_type === 'movie') navigate(`/movies/${it.media_id}`)
                        else if (it.media_type === 'tvshow') navigate(`/tvshows/${it.media_id}`)
                        // audiobook detail page not yet built — no-op.
                      }}
                      onFocus={row === lastRow ? scrollToBottom : undefined}
                    />
                  )
                })}
              </div>
            )}
          </main>
        )}
      </div>
    </FocusProvider>
  )
}

function BackButton({ onSelect, onFocus }: { onSelect: () => void; onFocus?: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    autoFocus: true,
    overrides: { left: 'sidebar' },
    onFocusChange: focused => { if (focused) onFocus?.() },
  })
  return (
    <button ref={ref} tabIndex={-1} onClick={onSelect} style={styles.backBtn}>
      ← Back
    </button>
  )
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
    gap: '2rem',
  },
  backBtn: {
    alignSelf: 'flex-start',
    display: 'inline-flex',
    alignItems: 'center',
    gap: '0.5rem',
    padding: '0.75rem 1.5rem',
    fontSize: '1rem',
    fontWeight: 600,
    color: 'var(--text)',
    background: 'var(--bg-elev)',
    borderRadius: 'var(--radius-md)',
  },
  header: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.75rem',
    maxWidth: '60rem',
  },
  name: {
    margin: 0,
    fontSize: '2.5rem',
    fontWeight: 700,
    letterSpacing: '-0.02em',
  },
  description: {
    margin: 0,
    fontSize: '1rem',
    lineHeight: 1.5,
    color: 'var(--text-muted)',
  },
  count: {
    color: 'var(--text-muted)',
    fontSize: '0.95rem',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: `repeat(${COLS}, minmax(0, 1fr))`,
    gap: '1.5rem 1.25rem',
  },
  empty: {
    color: 'var(--text-muted)',
    fontSize: '1rem',
  },
  error: {
    margin: 'var(--safe-y) var(--safe-x)',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}
