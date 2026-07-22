import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  api,
  type Audiobook,
  type Collection,
  type ContinueWatchingItem,
  type Movie,
  type NextUpItem,
  type RecentlyAddedItem,
  type TVShow,
} from '../api'
import { FocusProvider } from '../hooks/useFocus'
import { PosterCard } from '../components/PosterCard'
import { Row } from '../components/Row'
import { Sidebar } from '../components/Sidebar'
import { CollectionCard } from '../components/CollectionCard'
import { HeroBanner } from '../components/HeroBanner'

// Carousel caps — Continue Watching and Collections are short-form
// lists; Recently Added is the long tail and tends to be the row the
// user actually scrolls through, so it gets 32.
const SHORT_ROW = 16
const LONG_ROW  = 32

export default function HomePage() {
  const navigate = useNavigate()
  const [cont, setCont] = useState<ContinueWatchingItem[]>([])
  const [nextUp, setNextUp] = useState<NextUpItem[]>([])
  const [movies, setMovies] = useState<Movie[]>([])
  const [shows, setShows] = useState<TVShow[]>([])
  const [audiobooks, setAudiobooks] = useState<Audiobook[]>([])
  const [collections, setCollections] = useState<Collection[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    // The dedicated /recently-added endpoint caps at 10 mixed items
    // total, which leaves only a couple of shows after filtering. We
    // instead pull each type independently via the paged list endpoint
    // sorted by added-date, giving us full LONG_ROW worth per rail.
    Promise.all([
      api.getContinueWatching(),
      api.getNextUp(SHORT_ROW).catch(() => [] as NextUpItem[]),
      api.listMoviesPaged({ page: 1, limit: LONG_ROW, sort: 'added', order: 'desc' }),
      api.listTVShowsPaged({ page: 1, limit: LONG_ROW, sort: 'added', order: 'desc' }),
      api.listAudiobooksPaged({ page: 1, limit: LONG_ROW, sort: 'added', order: 'desc' })
        .catch(() => ({ items: [] as Audiobook[], total: 0 })),
      api.listCollections().catch(() => [] as Collection[]),
    ])
      .then(([c, n, m, s, a, col]) => {
        if (!alive) return
        setCont(c)
        setNextUp(n)
        setMovies(m.items)
        setShows(s.items)
        setAudiobooks(a.items)
        setCollections(col)
      })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
    return () => { alive = false }
  }, [])

  const contRow         = useMemo(() => cont.slice(0, SHORT_ROW), [cont])
  const nextUpRow       = useMemo(() => nextUp.slice(0, SHORT_ROW), [nextUp])
  const collectionsRow  = useMemo(() => collections.slice(0, SHORT_ROW), [collections])
  const recentMovies     = useMemo(() => movies.slice(0, LONG_ROW), [movies])
  const recentShows      = useMemo(() => shows.slice(0, LONG_ROW), [shows])
  const recentAudiobooks = useMemo(() => audiobooks.slice(0, LONG_ROW), [audiobooks])
  // Hero items are merged from both type-specific feeds and re-sorted
  // by created_at so the rotation matches "newest first" regardless of
  // type. Shaped as RecentlyAddedItem so HeroBanner doesn't need to
  // care about which endpoint produced each item.
  const heroItems = useMemo<RecentlyAddedItem[]>(() => {
    const merged: RecentlyAddedItem[] = [
      ...movies.map(m => ({
        id: m.id,
        media_type: 'movie' as const,
        title: m.title,
        year: m.year,
        description: m.description,
        genres: m.genres,
        rating: m.rating,
        poster_path: m.poster_path,
        backdrop_path: m.backdrop_path,
        file_path: m.file_path,
        added_at: m.created_at,
      })),
      ...shows.map(s => ({
        id: s.id,
        media_type: 'tvshow' as const,
        title: s.title,
        year: s.year,
        description: s.description,
        genres: s.genres,
        rating: s.rating,
        poster_path: s.poster_path,
        backdrop_path: s.backdrop_path,
        file_path: '',
        added_at: s.created_at,
      })),
    ]
    merged.sort((a, b) => b.added_at.localeCompare(a.added_at))
    return merged.slice(0, SHORT_ROW)
  }, [movies, shows])

  const mainRef = useRef<HTMLElement>(null)
  const scrollToBottom = () =>
    mainRef.current?.scrollTo({ top: mainRef.current.scrollHeight, behavior: 'smooth' })
  const scrollToTop = () =>
    mainRef.current?.scrollTo({ top: 0, behavior: 'smooth' })

  // Identify which row renders first / last so the top row can scroll
  // to the top (revealing the hero) and the bottom row can scroll to
  // the bottom. Order matches the conditional-render below.
  // Next Up sits ABOVE Continue Watching, so it becomes the topRow
  // candidate when populated. The render order below has to match this.
  const topRow =
    nextUpRow.length       > 0 ? 'next'    :
    contRow.length         > 0 ? 'cont'    :
    collectionsRow.length  > 0 ? 'collect' :
    recentMovies.length    > 0 ? 'movies'  :
    recentShows.length     > 0 ? 'shows'   :
    recentAudiobooks.length > 0 ? 'books'  : null
  const bottomRow =
    recentAudiobooks.length > 0 ? 'books'  :
    recentShows.length     > 0 ? 'shows'   :
    recentMovies.length    > 0 ? 'movies'  :
    collectionsRow.length  > 0 ? 'collect' :
    contRow.length         > 0 ? 'cont'    :
    nextUpRow.length       > 0 ? 'next'    : null
  // Same row both top and bottom? Prefer top so the hero stays visible.
  const focusScroll = (row: string) =>
    row === topRow ? scrollToTop : row === bottomRow ? scrollToBottom : undefined

  return (
    <FocusProvider backFocusesTag="sidebar">
      <div style={styles.page}>
        <Sidebar />

        <main ref={mainRef} style={styles.main}>
          {error && <div style={styles.error}>{error}</div>}

          {heroItems.length > 0 && (
            <HeroBanner
              items={heroItems}
              onMoreInfo={item => {
                if (item.media_type === 'movie') navigate(`/movies/${item.id}`)
                else navigate(`/tvshows/${item.id}`)
              }}
            />
          )}

          {nextUpRow.length > 0 && (
            <Row title="Next Up">
              {nextUpRow.map((item, i) => (
                <PosterCard
                  key={item.media_id}
                  title={item.title}
                  subtitle={
                    // "Show · S02E05" — the episode is the primary
                    // action, the show is the anchor context.
                    `${item.show_title} · S${String(item.season_number).padStart(2, '0')}E${String(item.episode_number).padStart(2, '0')}`
                  }
                  imageSrc={item.backdrop_path || item.poster_path || undefined}
                  aspect="landscape"
                  autoFocus={i === 0 && topRow === 'next'}
                  overrides={i === 0 ? { left: 'sidebar' } : undefined}
                  onFocus={focusScroll('next')}
                  onSelect={() => {
                    // Same as Continue Watching's episode branch: jump
                    // straight into the player at the next episode. The
                    // player fetches saved progress and starts at 0 for
                    // a never-watched episode.
                    //
                    // TODO(next-up): add a dismiss gesture on TV once
                    // useFocus grows long-press support. For now users
                    // can dismiss via river-web and the change flows
                    // through the DB.
                    navigate(
                      `/tvshows/${item.show_id}/seasons/${item.season_id}/episodes/${item.media_id}/watch`,
                    )
                  }}
                />
              ))}
            </Row>
          )}

          {contRow.length > 0 && (
            <Row title="Continue Watching">
              {contRow.map((item, i) => (
                <PosterCard
                  key={`${item.media_type}:${item.media_id}`}
                  title={item.title}
                  subtitle={item.show_title}
                  imageSrc={item.backdrop_path || item.poster_path || undefined}
                  aspect="landscape"
                  autoFocus={i === 0 && topRow === 'cont'}
                  overrides={i === 0 ? { left: 'sidebar' } : undefined}
                  onFocus={focusScroll('cont')}
                  onSelect={() => {
                    // Continue Watching selects go straight to the
                    // player, not the detail page — the whole point of
                    // this row is "resume where I left off". The player
                    // fetches saved progress and seeks on mount (see
                    // PlayerScreen), so we only need to land on the
                    // watch route with the right IDs.
                    //
                    // media_type is 'movie' | 'episode' | 'chapter'.
                    // For episodes we need show_id + season_id to build
                    // the nested watch URL; for chapters we need
                    // audiobook_id. If any of those are missing (stale
                    // record, pre-migration data) fall back to the
                    // detail page rather than silently doing nothing.
                    if (item.media_type === 'movie') {
                      navigate(`/movies/${item.media_id}/watch`)
                    } else if (item.media_type === 'episode' && item.show_id && item.season_id) {
                      navigate(`/tvshows/${item.show_id}/seasons/${item.season_id}/episodes/${item.media_id}/watch`)
                    } else if (item.media_type === 'episode' && item.show_id) {
                      navigate(`/tvshows/${item.show_id}`)
                    } else if (item.media_type === 'chapter' && item.audiobook_id) {
                      navigate(`/audiobooks/${item.audiobook_id}/chapters/${item.media_id}/listen`)
                    }
                  }}
                />
              ))}
            </Row>
          )}

          {collectionsRow.length > 0 && (
            <Row title="Collections">
              {collectionsRow.map((col, i) => (
                <CollectionCard
                  key={col.id}
                  name={col.name}
                  itemCount={col.item_count}
                  covers={col.covers ?? []}
                  autoFocus={i === 0 && topRow === 'collect'}
                  overrides={i === 0 ? { left: 'sidebar' } : undefined}
                  onFocus={focusScroll('collect')}
                  onSelect={() => navigate(`/collections/${col.id}`)}
                />
              ))}
            </Row>
          )}

          {recentMovies.length > 0 && (
            <Row title="Recently Added Movies">
              {recentMovies.map((item, i) => (
                <PosterCard
                  key={`movie:${item.id}`}
                  title={item.title}
                  subtitle={item.year ? String(item.year) : undefined}
                  imageSrc={item.poster_path || undefined}
                  autoFocus={i === 0 && topRow === 'movies'}
                  overrides={i === 0 ? { left: 'sidebar' } : undefined}
                  onFocus={focusScroll('movies')}
                  onSelect={() => navigate(`/movies/${item.id}`)}
                />
              ))}
            </Row>
          )}

          {recentShows.length > 0 && (
            <Row title="Recently Added Shows">
              {recentShows.map((item, i) => (
                <PosterCard
                  key={`tvshow:${item.id}`}
                  title={item.title}
                  subtitle={item.year ? String(item.year) : undefined}
                  imageSrc={item.poster_path || undefined}
                  autoFocus={i === 0 && topRow === 'shows'}
                  overrides={i === 0 ? { left: 'sidebar' } : undefined}
                  onFocus={focusScroll('shows')}
                  onSelect={() => navigate(`/tvshows/${item.id}`)}
                />
              ))}
            </Row>
          )}

          {recentAudiobooks.length > 0 && (
            <Row title="Recently Added Audiobooks">
              {recentAudiobooks.map((item, i) => (
                <PosterCard
                  key={`audiobook:${item.id}`}
                  title={item.title}
                  subtitle={item.author || (item.year ? String(item.year) : undefined)}
                  imageSrc={item.cover_path || undefined}
                  aspect="square"
                  autoFocus={i === 0 && topRow === 'books'}
                  overrides={i === 0 ? { left: 'sidebar' } : undefined}
                  onFocus={focusScroll('books')}
                  onSelect={() => navigate(`/audiobooks/${item.id}`)}
                />
              ))}
            </Row>
          )}
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
    display: 'flex',
    flexDirection: 'column',
    gap: '2rem',
    paddingTop: 'var(--safe-y)',
    paddingBottom: 'var(--safe-y)',
    scrollbarWidth: 'none',
  },
  error: {
    margin: '0 var(--safe-x)',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}
