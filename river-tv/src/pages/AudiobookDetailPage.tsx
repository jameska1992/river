import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  api,
  type Audiobook,
  type AudiobookChapter,
  type SimilarItem,
  type WatchlistItem,
} from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'
import { Sidebar } from '../components/Sidebar'
import { Row } from '../components/Row'
import { EpisodeCard } from '../components/EpisodeCard'
import { PosterCard } from '../components/PosterCard'
import { imageUrl } from '../util/imageUrl'

export default function AudiobookDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [book, setBook] = useState<Audiobook | null>(null)
  const [chapters, setChapters] = useState<AudiobookChapter[]>([])
  const [similar, setSimilar] = useState<SimilarItem[]>([])
  const [error, setError] = useState<string | null>(null)
  const [watchlistId, setWatchlistId] = useState<string | null>(null)
  const pageRef = useRef<HTMLDivElement>(null)

  const scrollToTop = () =>
    pageRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
  const scrollToBottom = () =>
    pageRef.current?.scrollTo({ top: pageRef.current.scrollHeight, behavior: 'smooth' })

  useEffect(() => {
    if (!id) return
    let alive = true
    Promise.all([
      api.getAudiobook(id),
      api.listChapters(id).catch(() => [] as AudiobookChapter[]),
      api.getWatchlist().catch(() => [] as WatchlistItem[]),
      api.getSimilarAudiobooks(id).catch(() => [] as SimilarItem[]),
    ])
      .then(([b, ch, wl, sim]) => {
        if (!alive) return
        setBook(b)
        setChapters([...ch].sort((a, b) => a.number - b.number))
        setWatchlistId(wl.find(w => w.media_type === 'audiobook' && w.media_id === id)?.id ?? null)
        setSimilar(sim)
      })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
    return () => { alive = false }
  }, [id])

  async function toggleWatchlist() {
    if (!id) return
    const previous = watchlistId
    if (previous) {
      setWatchlistId(null)
      try { await api.removeFromWatchlist(previous) }
      catch { setWatchlistId(previous) }
    } else {
      setWatchlistId('pending')
      try {
        const created = await api.addToWatchlist('audiobook', id)
        setWatchlistId(created.id)
      } catch { setWatchlistId(null) }
    }
  }

  return (
    <FocusProvider backFocusesTag="sidebar" onBack={() => navigate(-1)}>
      <div ref={pageRef} style={styles.page}>
        <Sidebar />

        {error && <div style={styles.error}>{error}</div>}

        {book && (
          <main style={styles.main}>
            <header style={styles.header}>
              <div style={styles.cover}>
                {imageUrl(book.cover_path) ? (
                  <img src={imageUrl(book.cover_path)} alt={book.title} decoding="async" style={styles.coverImg} />
                ) : (
                  <div style={styles.coverFallback}>{book.title.charAt(0)}</div>
                )}
              </div>

              <div style={styles.info}>
                <h1 style={styles.title}>{book.title}</h1>
                {book.author && <div style={styles.author}>{book.author}</div>}

                <div style={styles.attrs}>
                  {book.narrator && <span>Narrated by {book.narrator}</span>}
                  {book.year > 0 && <span>{book.year}</span>}
                  {book.genre && <span>{book.genre}</span>}
                  {book.duration > 0 && <span>{formatDuration(book.duration)}</span>}
                </div>

                {book.description && (
                  <p style={styles.description}>{book.description}</p>
                )}

                <div style={styles.actions}>
                  <ToggleButton
                    label={watchlistId ? '✓ In Watchlist' : '+ Watchlist'}
                    active={!!watchlistId}
                    autoFocus
                    onSelect={() => void toggleWatchlist()}
                    onFocus={scrollToTop}
                  />
                  <BackButton onSelect={() => navigate(-1)} onFocus={scrollToTop} />
                </div>
              </div>
            </header>

            {chapters.length > 0 && (
              <section style={styles.chaptersSection}>
                <Row title={`Chapters · ${chapters.length}`}>
                  {chapters.map(ch => (
                    <EpisodeCard
                      key={ch.id}
                      number={ch.number}
                      title={ch.title || `Chapter ${ch.number}`}
                      runtime={ch.duration > 0 ? Math.round(ch.duration / 60) : undefined}
                      available={!!ch.file_path}
                      onSelect={() => navigate(`/audiobooks/${id}/chapters/${ch.id}/listen`)}
                    />
                  ))}
                </Row>
                {/* Cast/credits-style row hook for future "About the author" */}
                <div style={styles.spacer} />
              </section>
            )}

            {similar.length > 0 && (
              <section style={styles.chaptersSection}>
                <Row title="More like this">
                  {similar.map((item, i) => (
                    <PosterCard
                      key={item.id}
                      title={item.title}
                      subtitle={item.year ? String(item.year) : undefined}
                      imageSrc={item.backdrop_path || item.poster_path || undefined}
                      aspect="landscape"
                      overrides={i === 0 ? { left: 'sidebar' } : undefined}
                      onFocus={scrollToBottom}
                      onSelect={() => navigate(similarRouteFor(item))}
                    />
                  ))}
                </Row>
              </section>
            )}
          </main>
        )}
      </div>
    </FocusProvider>
  )
}

function similarRouteFor(item: SimilarItem): string {
  switch (item.type) {
    case 'movie':     return `/movies/${item.id}`
    case 'tvshow':    return `/tvshows/${item.id}`
    case 'audiobook': return `/audiobooks/${item.id}`
  }
}

function BackButton({ onSelect, onFocus }: { onSelect: () => void; onFocus?: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    overrides: { left: 'sidebar' },
    onFocusChange: focused => { if (focused) onFocus?.() },
  })
  return (
    <button ref={ref} tabIndex={-1} onClick={onSelect} style={{ ...styles.btn, ...styles.btnSecondary }}>
      ← Back
    </button>
  )
}

function ToggleButton({
  label, active, autoFocus, onSelect, onFocus,
}: {
  label: string
  active: boolean
  autoFocus?: boolean
  onSelect: () => void
  onFocus?: () => void
}) {
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    autoFocus,
    overrides: { left: 'sidebar' },
    onFocusChange: focused => { if (focused) onFocus?.() },
  })
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={{ ...styles.btn, ...(active ? styles.btnToggleActive : styles.btnSecondary) }}
    >
      {label}
    </button>
  )
}

function formatDuration(seconds: number): string {
  if (!isFinite(seconds) || seconds <= 0) return ''
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
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
  header: {
    display: 'flex',
    gap: '3rem',
    alignItems: 'flex-start',
  },
  cover: {
    flex: '0 0 auto',
    width: '18rem',
    aspectRatio: '1 / 1',
    borderRadius: 'var(--radius-lg)',
    overflow: 'hidden',
    background: 'var(--bg-elev-2)',
    boxShadow: '0 1rem 3rem rgba(0,0,0,0.5)',
  },
  coverImg: {
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  },
  coverFallback: {
    width: '100%',
    height: '100%',
    display: 'grid',
    placeItems: 'center',
    fontSize: '5rem',
    color: 'var(--text-muted)',
  },
  info: {
    flex: 1,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
    maxWidth: '50rem',
  },
  title: {
    margin: 0,
    fontSize: '2.5rem',
    fontWeight: 700,
    letterSpacing: '-0.02em',
    lineHeight: 1.1,
  },
  author: {
    fontSize: '1.2rem',
    color: 'var(--text-muted)',
  },
  attrs: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: '1rem',
    color: 'var(--text-muted)',
    fontSize: '1rem',
  },
  description: {
    margin: 0,
    fontSize: '1.05rem',
    lineHeight: 1.5,
    color: 'var(--text)',
  },
  actions: {
    display: 'flex',
    gap: '1rem',
    marginTop: '0.5rem',
  },
  btn: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: '0.5rem',
    padding: '0.9rem 1.75rem',
    borderRadius: 'var(--radius-md)',
    fontSize: '1.05rem',
    fontWeight: 600,
  },
  btnSecondary: {
    background: 'var(--bg-elev)',
    color: 'var(--text)',
  },
  btnToggleActive: {
    background: 'var(--accent-soft)',
    color: 'var(--text)',
    border: '1px solid var(--accent)',
  },
  chaptersSection: {
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
  },
  spacer: { display: 'none' },
  error: {
    margin: 'var(--safe-y) var(--safe-x)',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}
