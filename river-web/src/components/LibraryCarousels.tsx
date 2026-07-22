import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  RiArrowRightSLine, RiArrowLeftSLine, RiArrowRightLine,
  RiFilmLine, RiTv2Line, RiMusicLine, RiHeadphoneLine,
} from 'react-icons/ri'
import type { ReactNode } from 'react'
import { useLibraries } from '../context/LibrariesContext'
import { useWatchlist } from '../context/WatchlistContext'
import { api } from '../api'
import type { Library } from '../api'
import { MediaCard } from './MediaCard'
import styles from './LibraryCarousels.module.css'

interface CardItem {
  id: string
  title: string
  subtitle?: string
  imageSrc?: string
  fallbackIcon: ReactNode
  to: string
  badge?: string
  rating?: string
  playType?: 'movie' | 'tvshow'
  playTo?: string
  // Only set for media types whose list payload exposes a file readiness
  // signal (movies). Undefined means "unknown / assume ready".
  playNotReady?: boolean
  progressRatio?: number
  watchlistType?: 'movie' | 'tvshow' | 'audiobook'
  // Set only for TV-show rows so the per-show watched indicator
  // (fully-watched = all episodes completed) renders on the card.
  completed?: boolean
}

// ── Public component ──────────────────────────────────────

export function LibraryCarousels() {
  const { libraries, isLoading, fetch } = useLibraries()

  useEffect(() => { void fetch() }, [fetch])

  if (isLoading && libraries.length === 0) return <Skeletons />
  if (libraries.length === 0) return null

  return (
    <div className={styles.carousels}>
      {libraries.map(lib => (
        <LibraryRow key={lib.id} library={lib} />
      ))}
    </div>
  )
}

// ── Per-library row ───────────────────────────────────────

function LibraryRow({ library }: { library: Library }) {
  const [items, setItems] = useState<CardItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([
      fetchItems(library),
      fetchProgressInfo(library.type),
      library.type === 'tvshow' ? fetchShowWatchedMap(library.id) : Promise.resolve(new Map<string, boolean>()),
    ])
      .then(([cardItems, progressInfo, watchedMap]) => {
        setItems(cardItems.map(item => ({
          ...item,
          progressRatio: progressInfo.ratios.get(item.id),
          completed:
            library.type === 'tvshow'
              ? (watchedMap.get(item.id) === true)
              : library.type === 'movie'
                ? (progressInfo.completed.get(item.id) === true)
                : undefined,
        })))
      })
      .catch(() => fetchItems(library).then(setItems).catch(() => {}))
      .finally(() => setLoading(false))
  }, [library.id, library.type]) // eslint-disable-line react-hooks/exhaustive-deps

  // Optimistic show-level watched toggle. Mirrors the library list page —
  // cascades to every episode on the server, also clears the carousel's
  // partial-progress bar when unmarking since the server is dropping the
  // episode rows anyway.
  const toggleShowWatched = useCallback((showId: string) => {
    let next = true
    setItems(prev => prev.map(item => {
      if (item.id !== showId) return item
      next = !(item.completed === true)
      return { ...item, completed: next, progressRatio: next ? item.progressRatio : undefined }
    }))
    api.setShowCompleted(showId, next).catch(() => {
      fetchShowWatchedMap(library.id).then(watchedMap => {
        setItems(prev => prev.map(item => ({
          ...item,
          completed: library.type === 'tvshow' ? (watchedMap.get(item.id) === true) : item.completed,
        })))
      }).catch(() => {})
    })
  }, [library.id, library.type])

  // Optimistic movie watched toggle. Marking watched stamps the card as
  // completed (and pegs the progress bar so a previously-partial movie
  // doesn't keep its bar AND a check at once); unmarking clears both.
  // On failure we re-pull movie progress to resync.
  const toggleMovieWatched = useCallback((movieId: string) => {
    let next = true
    setItems(prev => prev.map(item => {
      if (item.id !== movieId) return item
      next = !(item.completed === true)
      return {
        ...item,
        completed: next,
        progressRatio: next ? 1 : undefined,
      }
    }))
    api.setProgressCompleted('movie', movieId, next).catch(() => {
      fetchProgressInfo('movie').then(info => {
        setItems(prev => prev.map(item => ({
          ...item,
          progressRatio: info.ratios.get(item.id) ?? item.progressRatio,
          completed: info.completed.get(item.id) === true,
        })))
      }).catch(() => {})
    })
  }, [])

  if (!loading && items.length === 0) return null

  return (
    <section className={styles.row}>
      <div className={styles.rowHeader}>
        <Link to={`/library/${library.id}`} className={styles.rowTitleLink}>
          <span className={`headline-sm ${styles.rowTitle}`}>{library.name}</span>
          <RiArrowRightLine size={18} className={styles.rowTitleArrow} />
        </Link>
      </div>

      {loading ? (
        <SkeletonTrack />
      ) : (
        <Carousel
          items={items}
          onCompletedToggle={
            library.type === 'tvshow'
              ? toggleShowWatched
              : library.type === 'movie'
                ? toggleMovieWatched
                : undefined
          }
        />
      )}
    </section>
  )
}

// ── Carousel ──────────────────────────────────────────────

function Carousel({ items, onCompletedToggle }: { items: CardItem[]; onCompletedToggle?: (id: string) => void }) {
  const trackRef = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)
  const navigate = useNavigate()
  const [loadingPlayId, setLoadingPlayId] = useState<string | null>(null)
  const { isInWatchlist, toggle } = useWatchlist()

  const playItem = async (item: CardItem) => {
    if (item.playTo) {
      navigate(item.playTo)
      return
    }
    if (!item.playType) return
    setLoadingPlayId(item.id)
    try {
      if (item.playType === 'movie') {
        navigate(`/movie/${item.id}/watch`)
      } else {
        const { season_id, episode_id } = await api.getNextEpisode(item.id)
        navigate(`/show/${item.id}/season/${season_id}/episode/${episode_id}/watch`)
      }
    } catch {
      navigate(item.to)
    } finally {
      setLoadingPlayId(null)
    }
  }

  const syncArrows = () => {
    const el = trackRef.current
    if (!el) return
    setCanLeft(el.scrollLeft > 4)
    setCanRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 4)
  }

  useEffect(() => {
    syncArrows()
    const el = trackRef.current
    if (!el) return
    const ro = new ResizeObserver(syncArrows)
    ro.observe(el)
    return () => ro.disconnect()
  }, [items])

  const scroll = (dir: -1 | 1) => {
    const el = trackRef.current
    if (!el) return
    el.scrollBy({ left: el.clientWidth * 0.8 * dir, behavior: 'smooth' })
  }

  return (
    <div className={styles.carousel}>
      {/* Edge fade — left */}
      {canLeft && <div className={`${styles.fade} ${styles.fadeLeft}`} />}
      {/* Edge fade — right */}
      {canRight && <div className={`${styles.fade} ${styles.fadeRight}`} />}

      {/* Arrows */}
      {canLeft && (
        <button
          className={`${styles.arrow} ${styles.arrowLeft}`}
          onClick={() => scroll(-1)}
          aria-label="Scroll left"
        >
          <RiArrowLeftSLine size={26} />
        </button>
      )}
      {canRight && (
        <button
          className={`${styles.arrow} ${styles.arrowRight}`}
          onClick={() => scroll(1)}
          aria-label="Scroll right"
        >
          <RiArrowRightSLine size={26} />
        </button>
      )}

      {/* Track */}
      <div
        ref={trackRef}
        className={styles.track}
        onScroll={syncArrows}
      >
        {items.map(item => (
          <div key={item.id} className={styles.item}>
            <MediaCard
              title={item.title}
              subtitle={item.subtitle}
              imageSrc={item.imageSrc}
              fallbackIcon={item.fallbackIcon}
              badge={item.badge}
              rating={item.rating}
              to={item.to}
              onPlay={(item.playType || item.playTo) ? () => playItem(item) : undefined}
              playLoading={loadingPlayId === item.id}
              playNotReady={item.playNotReady}
              progressRatio={item.progressRatio}
              completed={item.completed}
              inWatchlist={item.watchlistType ? isInWatchlist(item.watchlistType, item.id) : undefined}
              onWatchlistToggle={item.watchlistType ? () => toggle(item.watchlistType!, item.id) : undefined}
              onCompletedToggle={onCompletedToggle ? () => onCompletedToggle(item.id) : undefined}
            />
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Data fetching ─────────────────────────────────────────

async function fetchItems(library: Library): Promise<CardItem[]> {
  switch (library.type) {
    case 'movie': {
      const movies = await api.listMovies({ library_id: library.id, limit: 20, sort: 'added', order: 'desc' })
      return movies.map(m => ({
        id: m.id,
        title: m.title,
        subtitle: m.year ? String(m.year) : undefined,
        imageSrc: m.poster_path || undefined,
        fallbackIcon: <RiFilmLine />,
        to: `/movie/${m.id}`,
        rating: m.rating ? m.rating.toFixed(1) : undefined,
        playType: 'movie' as const,
        playNotReady: !m.file_path,
        watchlistType: 'movie' as const,
      }))
    }
    case 'tvshow': {
      const shows = await api.listTVShows({ library_id: library.id, limit: 20, sort: 'added', order: 'desc' })
      return shows.map(s => ({
        id: s.id,
        title: s.title,
        subtitle: s.year ? String(s.year) : undefined,
        imageSrc: s.poster_path || undefined,
        fallbackIcon: <RiTv2Line />,
        to: `/show/${s.id}`,
        badge: s.status || undefined,
        playType: 'tvshow' as const,
        watchlistType: 'tvshow' as const,
      }))
    }
    case 'music': {
      const albums = await api.listAlbums({ library_id: library.id, limit: 20, sort: 'added', order: 'desc' })
      return albums.map(a => ({
        id: a.id,
        title: a.title,
        subtitle: a.year ? String(a.year) : undefined,
        imageSrc: a.cover_path || undefined,
        fallbackIcon: <RiMusicLine />,
        to: `/album/${a.id}`,
        playTo: `/album/${a.id}/play`,
      }))
    }
    case 'audiobook': {
      const books = await api.listAudiobooks({ library_id: library.id, limit: 20, sort: 'added', order: 'desc' })
      return books.map(b => ({
        id: b.id,
        title: b.title,
        subtitle: b.author || undefined,
        imageSrc: b.cover_path || undefined,
        fallbackIcon: <RiHeadphoneLine />,
        to: `/audiobook/${b.id}`,
        watchlistType: 'audiobook' as const,
      }))
    }
  }
}

// ── Show watched map ──────────────────────────────────

// fetchShowWatchedMap returns a per-show "fully watched" flag for the
// given TV library. A show is considered watched only when its episode
// count is > 0 (so a freshly-scanned show with no episode rows yet won't
// flash as "watched"). Bulk-loaded so the carousel doesn't fan out into
// N per-show progress lookups.
async function fetchShowWatchedMap(libraryID: string): Promise<Map<string, boolean>> {
  const states = await api.getShowWatchStates(libraryID)
  const map = new Map<string, boolean>()
  for (const s of states) map.set(s.show_id, s.total > 0 && s.completed >= s.total)
  return map
}

// ── Progress info ─────────────────────────────────────

interface ProgressInfo {
  // ratios are 0..1 inclusive — 1 means watched. Only present when the
  // user has actually started or finished the item.
  ratios: Map<string, number>
  // completed flags are populated for movies so the card's watched badge
  // can render in its filled state even when ratios doesn't have an entry
  // (e.g. just-marked-watched with no recorded position/duration). TV
  // rows leave this empty; show-level watched comes from
  // fetchShowWatchedMap.
  completed: Map<string, boolean>
}

async function fetchProgressInfo(type: Library['type']): Promise<ProgressInfo> {
  const ratios = new Map<string, number>()
  const completed = new Map<string, boolean>()
  if (type === 'movie') {
    const items = await api.getProgressAll('movie')
    for (const p of items) {
      const ratio = p.completed ? 1 : (p.duration > 0 ? p.position / p.duration : 0)
      if (ratio > 0) ratios.set(p.media_id, ratio)
      if (p.completed) completed.set(p.media_id, true)
    }
  } else if (type === 'tvshow') {
    const items = await api.getContinueWatching()
    for (const item of items) {
      if (item.show_id && !ratios.has(item.show_id) && item.duration > 0) {
        ratios.set(item.show_id, item.position / item.duration)
      }
    }
  }
  return { ratios, completed }
}

// ── Skeletons ─────────────────────────────────────────────

function SkeletonTrack() {
  return (
    <div className={styles.skeletonTrack}>
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className={`card card-portrait ${styles.skeletonCard}`} />
      ))}
    </div>
  )
}

function Skeletons() {
  return (
    <div className={styles.carousels}>
      {Array.from({ length: 2 }).map((_, i) => (
        <section key={i} className={styles.row}>
          <div className={`${styles.skeletonTitle}`} />
          <SkeletonTrack />
        </section>
      ))}
    </div>
  )
}
