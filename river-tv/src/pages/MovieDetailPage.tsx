import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type Credits, type Movie, type SimilarItem, type WatchlistItem } from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'
import { Sidebar } from '../components/Sidebar'
import { Row } from '../components/Row'
import { CastCard } from '../components/CastCard'
import { PosterCard } from '../components/PosterCard'
import { imageUrl } from '../util/imageUrl'

export default function MovieDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [movie, setMovie] = useState<Movie | null>(null)
  const [credits, setCredits] = useState<Credits | null>(null)
  const [similar, setSimilar] = useState<SimilarItem[]>([])
  const [error, setError] = useState<string | null>(null)
  // null = not on watchlist; string = id of the watchlist row (needed
  // for removal). Tracked separately from a boolean because the API
  // identifies entries by their own id, not by media id.
  const [watchlistId, setWatchlistId] = useState<string | null>(null)
  const [isWatched, setIsWatched] = useState(false)

  const pageRef = useRef<HTMLDivElement>(null)
  const scrollToTop = () =>
    pageRef.current?.scrollTo({ top: 0, behavior: 'smooth' })

  useEffect(() => {
    if (!id) return
    let alive = true
    Promise.all([
      api.getMovie(id),
      api.getMovieCredits(id).catch(() => null),
      api.getProgress('movie', id).catch(() => null),
      api.getWatchlist().catch(() => [] as WatchlistItem[]),
      api.getSimilarMovies(id).catch(() => [] as SimilarItem[]),
    ])
      .then(([m, c, p, wl, sim]) => {
        if (!alive) return
        setMovie(m)
        setCredits(c)
        setIsWatched(p?.completed === true)
        setWatchlistId(wl.find(w => w.media_type === 'movie' && w.media_id === id)?.id ?? null)
        setSimilar(sim)
      })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
    return () => { alive = false }
  }, [id])

  // Optimistic toggles. Rollback on error so a transient network blip
  // doesn't leave the UI lying about backend state.
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
        const created = await api.addToWatchlist('movie', id)
        setWatchlistId(created.id)
      } catch { setWatchlistId(null) }
    }
  }

  async function toggleWatched() {
    if (!id) return
    const next = !isWatched
    setIsWatched(next)
    try { await api.setProgressCompleted('movie', id, next) }
    catch { setIsWatched(!next) }
  }

  const directors = credits?.crew.filter(c => c.job === 'Director') ?? []
  const writers   = credits?.crew.filter(c => c.department === 'Writing') ?? []
  const cast      = credits?.cast ?? []

  return (
    <FocusProvider
      backFocusesTag="sidebar"
      onBack={() => navigate(-1)}
    >
      <div ref={pageRef} style={styles.page}>
        <Sidebar />

        {error && <div style={styles.error}>{error}</div>}

        {movie && (
          <>
            <section style={styles.hero}>
              {imageUrl(movie.backdrop_path, 'backdrop') && (
                <img src={imageUrl(movie.backdrop_path, 'backdrop')} alt="" decoding="async" style={styles.backdrop} />
              )}
              <div style={styles.scrim} />

              <div style={styles.heroContent}>
                <div style={styles.poster}>
                  {imageUrl(movie.poster_path) ? (
                    <img src={imageUrl(movie.poster_path)} alt={movie.title} decoding="async" style={styles.posterImg} />
                  ) : (
                    <div style={styles.posterFallback}>{movie.title.charAt(0)}</div>
                  )}
                </div>

                <div style={styles.info}>
                  <h1 style={styles.title}>{movie.title}</h1>
                  {movie.original_title && movie.original_title !== movie.title && (
                    <div style={styles.originalTitle}>{movie.original_title}</div>
                  )}

                  <div style={styles.attrs}>
                    {movie.year > 0 && <span>{movie.year}</span>}
                    {movie.runtime > 0 && <span>{formatRuntime(movie.runtime)}</span>}
                    {movie.rating > 0 && <span>★ {movie.rating.toFixed(1)}</span>}
                  </div>

                  {movie.genres.length > 0 && (
                    <div style={styles.genres}>
                      {movie.genres.map(g => (
                        <span key={g} style={styles.genre}>{g}</span>
                      ))}
                    </div>
                  )}

                  {movie.description && (
                    <p style={styles.description}>{movie.description}</p>
                  )}

                  <div style={styles.actions}>
                    <PlayButton
                      disabled={!movie.file_path}
                      onSelect={() => navigate(`/movies/${movie.id}/watch`)}
                      onFocus={scrollToTop}
                    />
                    <ToggleButton
                      label={watchlistId ? '✓ In Watchlist' : '+ Watchlist'}
                      active={!!watchlistId}
                      onSelect={() => void toggleWatchlist()}
                      onFocus={scrollToTop}
                    />
                    <ToggleButton
                      label={isWatched ? '✓ Watched' : 'Mark Watched'}
                      active={isWatched}
                      onSelect={() => void toggleWatched()}
                      onFocus={scrollToTop}
                    />
                    <BackButton onSelect={() => navigate(-1)} onFocus={scrollToTop} />
                  </div>

                  {!movie.file_path && (
                    <p style={styles.notReady}>File not yet available for streaming.</p>
                  )}

                  {(directors.length > 0 || writers.length > 0) && (
                    <div style={styles.creditsMeta}>
                      {directors.length > 0 && (
                        <div style={styles.creditsRow}>
                          <span style={styles.creditsLabel}>Director{directors.length > 1 ? 's' : ''}</span>
                          <span style={styles.creditsValue}>{directors.map(d => d.name).join(', ')}</span>
                        </div>
                      )}
                      {writers.length > 0 && (
                        <div style={styles.creditsRow}>
                          <span style={styles.creditsLabel}>Writer{writers.length > 1 ? 's' : ''}</span>
                          <span style={styles.creditsValue}>{writers.map(w => w.name).join(', ')}</span>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </section>

            {cast.length > 0 && (
              <section style={styles.creditsSection}>
                <Row title="Cast">
                  {cast.map(c => (
                    <CastCard
                      key={c.person_id}
                      name={c.name}
                      role={c.character}
                      photoUrl={c.profile_path || undefined}
                      onSelect={() => navigate(`/people/${c.person_id}`)}
                    />
                  ))}
                </Row>
              </section>
            )}

            {similar.length > 0 && (
              <section style={styles.creditsSection}>
                <Row title="More like this">
                  {similar.map((item, i) => (
                    <PosterCard
                      key={item.id}
                      title={item.title}
                      subtitle={item.year ? String(item.year) : undefined}
                      imageSrc={item.backdrop_path || item.poster_path || undefined}
                      aspect="landscape"
                      overrides={i === 0 ? { left: 'sidebar' } : undefined}
                      onSelect={() => navigate(similarRouteFor(item))}
                    />
                  ))}
                </Row>
              </section>
            )}
          </>
        )}
      </div>
    </FocusProvider>
  )
}

// similarRouteFor maps a SimilarItem back to the detail-page route for
// its type. Kept at file scope (rather than a shared util) because
// each of the three detail pages needs the same handful of lines and
// the mapping is trivial — a shared helper adds a level of indirection
// without saving many characters.
function similarRouteFor(item: SimilarItem): string {
  switch (item.type) {
    case 'movie':     return `/movies/${item.id}`
    case 'tvshow':    return `/tvshows/${item.id}`
    case 'audiobook': return `/audiobooks/${item.id}`
  }
}

function PlayButton({ disabled, onSelect, onFocus }: { disabled: boolean; onSelect: () => void; onFocus?: () => void }) {
  // First focus on the page — most likely action.
  const ref = useFocusable<HTMLButtonElement>(
    disabled ? undefined : onSelect,
    {
      autoFocus: true,
      overrides: { left: 'sidebar' },
      onFocusChange: focused => { if (focused) onFocus?.() },
    },
  )
  return (
    <button
      ref={ref}
      tabIndex={-1}
      disabled={disabled}
      onClick={disabled ? undefined : onSelect}
      style={{ ...styles.btn, ...styles.btnPrimary, ...(disabled ? styles.btnDisabled : {}) }}
    >
      ▶  Play
    </button>
  )
}

function BackButton({ onSelect, onFocus }: { onSelect: () => void; onFocus?: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    onFocusChange: focused => { if (focused) onFocus?.() },
  })
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={{ ...styles.btn, ...styles.btnSecondary }}
    >
      ← Back
    </button>
  )
}

function ToggleButton({
  label, active, onSelect, onFocus,
}: {
  label: string
  active: boolean
  onSelect: () => void
  onFocus?: () => void
}) {
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    onFocusChange: focused => { if (focused) onFocus?.() },
  })
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={{
        ...styles.btn,
        ...(active ? styles.btnToggleActive : styles.btnSecondary),
      }}
    >
      {label}
    </button>
  )
}

function formatRuntime(mins: number): string {
  const h = Math.floor(mins / 60)
  const m = mins % 60
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    height: '100vh',
    overflowY: 'auto',
    position: 'relative',
    paddingLeft: 'var(--sidebar-rail)',
    scrollbarWidth: 'none',
  },
  hero: {
    position: 'relative',
    // Sized by the poster + info column rather than the viewport so the
    // credits section below sits inside the visible area (or peeks just
    // below) rather than being totally hidden under the 100vh fold.
    overflow: 'hidden',
  },
  backdrop: {
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    objectFit: 'cover',
    zIndex: 0,
  },
  scrim: {
    position: 'absolute',
    inset: 0,
    // Two stacked gradients:
    //  - left-to-right scrim so the info column is readable over the
    //    backdrop on the left, while the right edge stays photographic.
    //  - bottom-to-top scrim that ends at fully opaque var(--bg) so the
    //    hero blends seamlessly into the credits section below it,
    //    instead of cutting off with a hard image edge.
    background:
      'linear-gradient(to right, rgba(19,19,19,0.95) 0%, rgba(19,19,19,0.85) 40%, rgba(19,19,19,0.6) 70%, rgba(19,19,19,0.3) 100%),' +
      'linear-gradient(to top, rgba(19,19,19,1) 0%, rgba(19,19,19,0.85) 15%, transparent 55%)',
    zIndex: 1,
  },
  heroContent: {
    position: 'relative',
    zIndex: 2,
    display: 'flex',
    gap: '3rem',
    // Top padding leaves room above the title; no bottom padding so the
    // Cast row that follows hugs the actions without a wide gap. The
    // detail page is meant to fit on a single 1080p screen without
    // scrolling.
    padding: 'var(--safe-y) var(--safe-x) 0',
    boxSizing: 'border-box',
    alignItems: 'flex-start',
  },
  poster: {
    flex: '0 0 auto',
    width: '20rem',
    aspectRatio: '2 / 3',
    borderRadius: 'var(--radius-lg)',
    overflow: 'hidden',
    background: 'var(--bg-elev-2)',
    boxShadow: '0 1rem 3rem rgba(0,0,0,0.5)',
  },
  posterImg: {
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  },
  posterFallback: {
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
    fontSize: '3rem',
    fontWeight: 700,
    letterSpacing: '-0.02em',
    lineHeight: 1.1,
  },
  originalTitle: {
    color: 'var(--text-muted)',
    fontSize: '1rem',
  },
  attrs: {
    display: 'flex',
    gap: '1.25rem',
    color: 'var(--text-muted)',
    fontSize: '1.1rem',
  },
  genres: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: '0.5rem',
  },
  genre: {
    padding: '0.25rem 0.75rem',
    fontSize: '0.85rem',
    background: 'rgba(255,255,255,0.1)',
    color: 'var(--text)',
    borderRadius: '999px',
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
    marginTop: '1rem',
  },
  btn: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: '0.5rem',
    padding: '0.9rem 1.75rem',
    borderRadius: 'var(--radius-md)',
    fontSize: '1.1rem',
    fontWeight: 600,
  },
  btnPrimary: {
    background: 'var(--accent)',
    color: 'var(--on-accent)',
  },
  btnSecondary: {
    background: 'rgba(255,255,255,0.16)',
    color: 'var(--text)',
    backdropFilter: 'blur(8px)',
  },
  btnToggleActive: {
    background: 'var(--accent-soft)',
    color: 'var(--text)',
    border: '1px solid var(--accent)',
  },
  btnDisabled: {
    opacity: 0.45,
    cursor: 'default',
  },
  notReady: {
    margin: 0,
    color: 'var(--text-muted)',
    fontSize: '0.95rem',
  },
  creditsSection: {
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
    // No top padding — sit immediately under the hero. Only bottom
    // safe-area for the TV-safe edge.
    padding: '0 0 var(--safe-y)',
  },
  creditsMeta: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    marginTop: '0.5rem',
  },
  creditsRow: {
    display: 'flex',
    gap: '1rem',
    alignItems: 'baseline',
  },
  creditsLabel: {
    color: 'var(--text-muted)',
    fontSize: '0.85rem',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
    minWidth: '6rem',
  },
  creditsValue: {
    color: 'var(--text)',
    fontSize: '1.05rem',
  },
  error: {
    position: 'relative',
    zIndex: 2,
    margin: 'var(--safe-y) var(--safe-x)',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}
