import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  api,
  type Credits,
  type Episode,
  type Season,
  type SimilarItem,
  type TVShow,
  type WatchlistItem,
} from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'
import { Sidebar } from '../components/Sidebar'
import { Row } from '../components/Row'
import { CastCard } from '../components/CastCard'
import { EpisodeCard } from '../components/EpisodeCard'
import { PosterCard } from '../components/PosterCard'
import { imageUrl } from '../util/imageUrl'

export default function TVShowDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [show, setShow] = useState<TVShow | null>(null)
  const [seasons, setSeasons] = useState<Season[]>([])
  const [credits, setCredits] = useState<Credits | null>(null)
  const [similar, setSimilar] = useState<SimilarItem[]>([])
  const [watchlistId, setWatchlistId] = useState<string | null>(null)
  const [isWatched, setIsWatched] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [selectedSeasonId, setSelectedSeasonId] = useState<string | null>(null)
  const [episodes, setEpisodes] = useState<Episode[]>([])
  const [episodesLoading, setEpisodesLoading] = useState(false)

  const pageRef = useRef<HTMLDivElement>(null)
  const scrollToTop = () =>
    pageRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
  const scrollToBottom = () =>
    pageRef.current?.scrollTo({ top: pageRef.current.scrollHeight, behavior: 'smooth' })

  // Initial load — show, seasons, credits, progress (show-level), watchlist.
  useEffect(() => {
    if (!id) return
    let alive = true
    Promise.all([
      api.getTVShow(id),
      api.listSeasons(id),
      api.getTVShowCredits(id).catch(() => null),
      api.getShowWatchState(id).catch(() => null),
      api.getWatchlist().catch(() => [] as WatchlistItem[]),
      api.getSimilarShows(id).catch(() => [] as SimilarItem[]),
    ])
      .then(([s, ss, c, watchState, wl, sim]) => {
        if (!alive) return
        setShow(s)
        const sorted = [...ss].sort((a, b) => a.number - b.number)
        setSeasons(sorted)
        setCredits(c)
        setIsWatched(!!watchState && watchState.total > 0 && watchState.completed >= watchState.total)
        setWatchlistId(wl.find(w => w.media_type === 'tvshow' && w.media_id === id)?.id ?? null)
        if (sorted.length > 0) setSelectedSeasonId(sorted[0].id)
        setSimilar(sim)
      })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
    return () => { alive = false }
  }, [id])

  // Fetch episodes whenever the selected season changes.
  useEffect(() => {
    if (!id || !selectedSeasonId) return
    let alive = true
    // eslint-disable-next-line react-hooks/set-state-in-effect -- data-fetching effect; loading state must flip synchronously when the selected season changes
    setEpisodesLoading(true)
    api.listEpisodes(id, selectedSeasonId)
      .then(eps => { if (alive) setEpisodes([...eps].sort((a, b) => a.number - b.number)) })
      .catch(() => { if (alive) setEpisodes([]) })
      .finally(() => { if (alive) setEpisodesLoading(false) })
    return () => { alive = false }
  }, [id, selectedSeasonId])

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
        const created = await api.addToWatchlist('tvshow', id)
        setWatchlistId(created.id)
      } catch { setWatchlistId(null) }
    }
  }

  async function toggleWatched() {
    if (!id) return
    const next = !isWatched
    setIsWatched(next)
    try { await api.setShowCompleted(id, next) }
    catch { setIsWatched(!next) }
  }

  async function playNext() {
    if (!id) return
    try {
      const { season_id, episode_id } = await api.getNextEpisode(id)
      navigate(`/tvshows/${id}/seasons/${season_id}/episodes/${episode_id}/watch`)
    } catch (e) {
      console.warn('no playable next episode', e)
    }
  }

  const cast = credits?.cast ?? []

  return (
    <FocusProvider backFocusesTag="sidebar" onBack={() => navigate(-1)}>
      <div ref={pageRef} style={styles.page}>
        <Sidebar />

        {error && <div style={styles.error}>{error}</div>}

        {show && (
          <>
            <section style={styles.hero}>
              {imageUrl(show.backdrop_path, 'backdrop') && (
                <img src={imageUrl(show.backdrop_path, 'backdrop')} alt="" decoding="async" style={styles.backdrop} />
              )}
              <div style={styles.scrim} />

              <div style={styles.heroContent}>
                <div style={styles.poster}>
                  {imageUrl(show.poster_path) ? (
                    <img src={imageUrl(show.poster_path)} alt={show.title} decoding="async" style={styles.posterImg} />
                  ) : (
                    <div style={styles.posterFallback}>{show.title.charAt(0)}</div>
                  )}
                </div>

                <div style={styles.info}>
                  <h1 style={styles.title}>{show.title}</h1>
                  {show.original_title && show.original_title !== show.title && (
                    <div style={styles.originalTitle}>{show.original_title}</div>
                  )}

                  <div style={styles.attrs}>
                    {show.year > 0 && <span>{show.year}</span>}
                    {show.status && <span>{show.status}</span>}
                    {show.rating > 0 && <span>★ {show.rating.toFixed(1)}</span>}
                    {seasons.length > 0 && (
                      <span>{seasons.length} {seasons.length === 1 ? 'season' : 'seasons'}</span>
                    )}
                  </div>

                  {show.genres.length > 0 && (
                    <div style={styles.genres}>
                      {show.genres.map(g => (
                        <span key={g} style={styles.genre}>{g}</span>
                      ))}
                    </div>
                  )}

                  {show.description && (
                    <p style={styles.description}>{show.description}</p>
                  )}

                  <div style={styles.actions}>
                    <PlayButton onSelect={() => void playNext()} onFocus={scrollToTop} />
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
                </div>
              </div>
            </section>

            {seasons.length > 0 && (
              <section style={styles.seasonsSection}>
                <div style={styles.seasonsBar}>
                  {seasons.map(s => (
                    <SeasonTab
                      key={s.id}
                      label={s.title || `Season ${s.number}`}
                      active={s.id === selectedSeasonId}
                      onSelect={() => setSelectedSeasonId(s.id)}
                    />
                  ))}
                </div>

                <Row title={seasonRowTitle(seasons, selectedSeasonId, episodes.length, episodesLoading)}>
                  {episodes.map(ep => (
                    <EpisodeCard
                      key={ep.id}
                      number={ep.number}
                      title={ep.title}
                      description={ep.description}
                      runtime={ep.runtime}
                      available={!!ep.file_path}
                      isSpecial={ep.is_special}
                      onSelect={() => {
                        if (id && selectedSeasonId) {
                          navigate(`/tvshows/${id}/seasons/${selectedSeasonId}/episodes/${ep.id}/watch`)
                        }
                      }}
                    />
                  ))}
                </Row>
              </section>
            )}

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
                      onFocus={scrollToBottom}
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
                      onFocus={scrollToBottom}
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

function similarRouteFor(item: SimilarItem): string {
  switch (item.type) {
    case 'movie':     return `/movies/${item.id}`
    case 'tvshow':    return `/tvshows/${item.id}`
    case 'audiobook': return `/audiobooks/${item.id}`
  }
}

function seasonRowTitle(seasons: Season[], selectedId: string | null, count: number, loading: boolean): string {
  const s = seasons.find(x => x.id === selectedId)
  if (!s) return 'Episodes'
  if (loading) return `Season ${s.number} • loading…`
  return `Season ${s.number} • ${count} ${count === 1 ? 'episode' : 'episodes'}`
}

function SeasonTab({
  label, active, onSelect,
}: {
  label: string
  active: boolean
  onSelect: () => void
}) {
  // tag the tab group so they all reach each other via spatial nav and
  // so Up from an episode card doesn't accidentally find a tab way
  // across the page via offline weight (kept tag-free is fine here
  // since tabs are inline with cards anyway).
  const ref = useFocusable<HTMLButtonElement>(onSelect)
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={{ ...styles.seasonTab, ...(active ? styles.seasonTabActive : {}) }}
    >
      {label}
    </button>
  )
}

function PlayButton({ onSelect, onFocus }: { onSelect: () => void; onFocus?: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    autoFocus: true,
    overrides: { left: 'sidebar' },
    onFocusChange: focused => { if (focused) onFocus?.() },
  })
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={{ ...styles.btn, ...styles.btnPrimary }}
    >
      ▶  Play Next
    </button>
  )
}

function BackButton({ onSelect, onFocus }: { onSelect: () => void; onFocus?: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    onFocusChange: focused => { if (focused) onFocus?.() },
  })
  return (
    <button ref={ref} tabIndex={-1} onClick={onSelect} style={{ ...styles.btn, ...styles.btnSecondary }}>
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
      style={{ ...styles.btn, ...(active ? styles.btnToggleActive : styles.btnSecondary) }}
    >
      {label}
    </button>
  )
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
  posterImg: { width: '100%', height: '100%', objectFit: 'cover' },
  posterFallback: {
    width: '100%', height: '100%',
    display: 'grid', placeItems: 'center',
    fontSize: '5rem', color: 'var(--text-muted)',
  },
  info: {
    flex: 1, minWidth: 0,
    display: 'flex', flexDirection: 'column', gap: '1rem',
    maxWidth: '50rem',
  },
  title: { margin: 0, fontSize: '3rem', fontWeight: 700, letterSpacing: '-0.02em', lineHeight: 1.1 },
  originalTitle: { color: 'var(--text-muted)', fontSize: '1rem' },
  attrs: { display: 'flex', gap: '1.25rem', color: 'var(--text-muted)', fontSize: '1.1rem' },
  genres: { display: 'flex', flexWrap: 'wrap', gap: '0.5rem' },
  genre: {
    padding: '0.25rem 0.75rem', fontSize: '0.85rem',
    background: 'rgba(255,255,255,0.1)', color: 'var(--text)',
    borderRadius: '999px',
  },
  description: { margin: 0, fontSize: '1.05rem', lineHeight: 1.5, color: 'var(--text)' },
  actions: { display: 'flex', gap: '1rem', marginTop: '1rem' },
  btn: {
    display: 'inline-flex', alignItems: 'center', gap: '0.5rem',
    padding: '0.9rem 1.75rem', borderRadius: 'var(--radius-md)',
    fontSize: '1.1rem', fontWeight: 600,
  },
  btnPrimary: { background: 'var(--accent)', color: 'var(--on-accent)' },
  btnSecondary: {
    background: 'rgba(255,255,255,0.16)', color: 'var(--text)',
    backdropFilter: 'blur(8px)',
  },
  btnToggleActive: {
    background: 'var(--accent-soft)', color: 'var(--text)',
    border: '1px solid var(--accent)',
  },

  seasonsSection: {
    display: 'flex', flexDirection: 'column',
    gap: '1rem',
    padding: '0.5rem 0 0',
  },
  seasonsBar: {
    display: 'flex',
    gap: '0.5rem',
    padding: '0 var(--safe-x)',
    flexWrap: 'wrap',
  },
  seasonTab: {
    padding: '0.5rem 1.1rem',
    fontSize: '0.95rem',
    fontWeight: 600,
    color: 'var(--text-muted)',
    background: 'var(--bg-elev)',
    borderRadius: '999px',
  },
  seasonTabActive: {
    background: 'var(--accent-soft)',
    color: 'var(--text)',
    border: '1px solid var(--accent)',
  },

  creditsSection: {
    display: 'flex', flexDirection: 'column',
    gap: '1rem',
    padding: '0 0 var(--safe-y)',
  },

  error: {
    position: 'relative', zIndex: 2,
    margin: 'var(--safe-y) var(--safe-x)',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)', color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}
