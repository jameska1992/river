import { useEffect, useState, useCallback } from 'react'
import { useParams, Navigate, useNavigate } from 'react-router-dom'
import {
  RiFilmLine, RiTv2Line, RiMusicLine, RiHeadphoneLine,
  RiArrowLeftSLine, RiArrowRightSLine,
  RiArrowLeftDoubleLine, RiArrowRightDoubleLine,
} from 'react-icons/ri'
import { useLibraries } from '../context/LibrariesContext'
import { useWatchlist } from '../context/WatchlistContext'
import { MediaCard } from '../components/MediaCard'
import { SortControl, type SortOption } from '../components/SortControl'
import { api } from '../api'
import type { Library, SortOrder, WatchProgress, ContinueWatchingItem } from '../api'
import { usePaginatedList } from '../hooks/usePaginatedList'
import styles from './LibraryPage.module.css'

// Sort preferences are persisted per library so a user's pick for one
// movie library doesn't follow them into another. The default is a
// sensible alphabetical sort; the server uses the same default when no
// sort query param is sent.
interface SortState { field: string; order: SortOrder }

function sortStorageKey(libraryId: string) {
  return `river:librarySort:${libraryId}`
}

function loadSort(libraryId: string, fallback: SortState): SortState {
  try {
    const raw = localStorage.getItem(sortStorageKey(libraryId))
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as Partial<SortState>
    return {
      field: typeof parsed.field === 'string' && parsed.field ? parsed.field : fallback.field,
      order: parsed.order === 'desc' ? 'desc' : 'asc',
    }
  } catch {
    return fallback
  }
}

function useSortPref(libraryId: string, fallback: SortState) {
  const [state, setState] = useState<SortState>(() => loadSort(libraryId, fallback))
  // Reset when the library changes — otherwise the hook would keep the
  // previous library's pick until the next setSort.
  // eslint-disable-next-line react-hooks/exhaustive-deps, react-hooks/set-state-in-effect -- resyncs sort pref from storage when the library id changes
  useEffect(() => { setState(loadSort(libraryId, fallback)) }, [libraryId])
  const setSort = useCallback((field: string, order: SortOrder) => {
    const next = { field, order }
    setState(next)
    try { localStorage.setItem(sortStorageKey(libraryId), JSON.stringify(next)) } catch { /* ignore */ }
  }, [libraryId])
  return [state, setSort] as const
}

// Page-size preference is global (not per-library): it reflects how much
// the user wants to see at once, not anything intrinsic to a library.
const PAGE_SIZE_OPTIONS = [50, 100, 150, 200] as const
const PAGE_SIZE_KEY = 'river:libraryPageSize'
const DEFAULT_PAGE_SIZE = 50

function loadPageSize(): number {
  try {
    const raw = localStorage.getItem(PAGE_SIZE_KEY)
    const n = raw ? parseInt(raw, 10) : NaN
    if ((PAGE_SIZE_OPTIONS as readonly number[]).includes(n)) return n
  } catch { /* ignore */ }
  return DEFAULT_PAGE_SIZE
}

function usePageSizePref() {
  const [pageSize, setPageSizeState] = useState<number>(() => loadPageSize())
  const setPageSize = useCallback((n: number) => {
    setPageSizeState(n)
    try { localStorage.setItem(PAGE_SIZE_KEY, String(n)) } catch { /* ignore */ }
  }, [])
  return [pageSize, setPageSize] as const
}

function PageSizeDropdown({ value, onChange }: { value: number; onChange: (n: number) => void }) {
  return (
    <div className={styles.sortControl}>
      <span className={`label-sm ${styles.sortLabel}`}>Per page</span>
      <select
        className={styles.sortSelect}
        value={value}
        onChange={e => onChange(parseInt(e.target.value, 10))}
      >
        {PAGE_SIZE_OPTIONS.map(n => (
          <option key={n} value={n}>{n}</option>
        ))}
      </select>
    </div>
  )
}

const MOVIE_SORT_OPTIONS: SortOption[] = [
  { value: 'title:asc',      label: 'Title (A–Z)'             },
  { value: 'title:desc',     label: 'Title (Z–A)'             },
  { value: 'year:desc',      label: 'Year (newest first)'     },
  { value: 'year:asc',       label: 'Year (oldest first)'     },
  { value: 'rating:desc',    label: 'Rating (high–low)'       },
  { value: 'rating:asc',     label: 'Rating (low–high)'       },
  { value: 'added:desc',     label: 'Recently added'          },
  { value: 'added:asc',      label: 'Oldest added'            },
]

const TV_SORT_OPTIONS = MOVIE_SORT_OPTIONS

const ALBUM_SORT_OPTIONS: SortOption[] = [
  { value: 'title:asc',      label: 'Title (A–Z)'             },
  { value: 'title:desc',     label: 'Title (Z–A)'             },
  { value: 'year:desc',      label: 'Year (newest first)'     },
  { value: 'year:asc',       label: 'Year (oldest first)'     },
  { value: 'added:desc',     label: 'Recently added'          },
  { value: 'added:asc',      label: 'Oldest added'            },
]

const AUDIOBOOK_SORT_OPTIONS: SortOption[] = [
  { value: 'title:asc',      label: 'Title (A–Z)'             },
  { value: 'title:desc',     label: 'Title (Z–A)'             },
  { value: 'author:asc',     label: 'Author (A–Z)'            },
  { value: 'author:desc',    label: 'Author (Z–A)'            },
  { value: 'year:desc',      label: 'Year (newest first)'     },
  { value: 'year:asc',       label: 'Year (oldest first)'     },
  { value: 'added:desc',     label: 'Recently added'          },
  { value: 'added:asc',      label: 'Oldest added'            },
]

function SortDropdown({ options, field, order, onChange }: {
  options: SortOption[]; field: string; order: SortOrder;
  onChange: (f: string, o: SortOrder) => void;
}) {
  return (
    <SortControl
      options={options}
      field={field}
      order={order}
      onChange={onChange}
      className={styles.sortControl}
      selectClassName={styles.sortSelect}
      labelClassName={styles.sortLabel}
    />
  )
}

export function LibraryPage() {
  const { id } = useParams<{ id: string }>()
  const { libraries, isLoading: libsLoading, fetch: fetchLibraries } = useLibraries()

  useEffect(() => {
    void fetchLibraries()
  }, [fetchLibraries])

  if (libsLoading) {
    return <LoadingGrid />
  }

  const library = libraries.find(l => l.id === id)

  if (!libsLoading && libraries.length > 0 && !library) {
    return <Navigate to="/" replace />
  }

  if (!library) return <LoadingGrid />

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className="headline-lg" style={{ margin: 0 }}>{library.name}</h1>
      </div>
      <TypedLibrary library={library} />
    </div>
  )
}

function TypedLibrary({ library }: { library: Library }) {
  switch (library.type) {
    case 'movie':     return <MovieLibrary libraryId={library.id} />
    case 'tvshow':    return <TVLibrary libraryId={library.id} />
    case 'music':     return <MusicLibrary libraryId={library.id} />
    case 'audiobook': return <AudiobookLibrary libraryId={library.id} />
  }
}

// ---------- Movies ----------

function MovieLibrary({ libraryId }: { libraryId: string }) {
  const navigate = useNavigate()
  const { isInWatchlist, toggle } = useWatchlist()
  const [progressMap, setProgressMap] = useState<Map<string, WatchProgress>>(new Map())
  const [sort, setSort] = useSortPref(libraryId, { field: 'title', order: 'asc' })
  const [pageSize, setPageSize] = usePageSizePref()

  useEffect(() => {
    api.getProgressAll('movie')
      .then(items => setProgressMap(new Map(items.map(p => [p.media_id, p]))))
      .catch(() => {})
  }, [])

  const fetchPage = useCallback(
    (page: number, limit: number) => api.listMoviesPaged({ library_id: libraryId, page, limit, sort: sort.field, order: sort.order }),
    [libraryId, sort.field, sort.order],
  )
  const { items: movies, total, page, totalPages, isLoading, error, goToPage } = usePaginatedList(fetchPage, pageSize)

  // Optimistic toggle in both directions: marking unwatched drops the
  // progress row, marking watched stamps a synthetic completed entry so
  // the card flips immediately. The server is the source of truth — on
  // failure we re-pull the full progress list to resync. We compute the
  // target value from the current progressMap *before* calling
  // setProgressMap; React doesn't run updater functions synchronously, so
  // deriving `next` inside the updater would leak the wrong value out to
  // the API call and silently send the inverse action.
  const handleToggleWatched = useCallback((movieId: string) => {
    const wasCompleted = progressMap.get(movieId)?.completed === true
    const next = !wasCompleted
    setProgressMap(prev => {
      const out = new Map(prev)
      if (next) {
        out.set(movieId, {
          ...(prev.get(movieId) ?? {
            id: '', user_id: '', media_type: 'movie', media_id: movieId,
            position: 0, duration: 0, created_at: '', updated_at: '',
          }),
          completed: true,
        })
      } else {
        out.delete(movieId)
      }
      return out
    })
    api.setProgressCompleted('movie', movieId, next).catch(() => {
      api.getProgressAll('movie')
        .then(items => setProgressMap(new Map(items.map(p => [p.media_id, p]))))
        .catch(() => {})
    })
  }, [progressMap])

  if (error) return <ErrorState message={error} />

  return (
    <>
      <div className={styles.toolbar}>
        <p className={`label-sm ${styles.count}`}>
          {isLoading ? 'Loading…' : formatRange(page, pageSize, movies.length, total, 'movie', 'movies')}
        </p>
        <Pagination page={page} totalPages={totalPages} isLoading={isLoading} onChange={goToPage} inline />
        <div className={styles.toolbarRight}>
          <PageSizeDropdown value={pageSize} onChange={setPageSize} />
          <SortDropdown options={MOVIE_SORT_OPTIONS} field={sort.field} order={sort.order} onChange={setSort} />
        </div>
      </div>
      {isLoading ? <LoadingGrid /> : movies.length === 0 ? <EmptyState label="No movies found" /> : (
      <div className={styles.grid}>
        {movies.map(m => {
          const prog = progressMap.get(m.id)
          const progressRatio = prog
            ? (prog.completed ? 1 : prog.duration > 0 ? prog.position / prog.duration : undefined)
            : undefined
          return (
            <MediaCard
              key={m.id}
              title={m.title}
              subtitle={m.year ? String(m.year) : undefined}
              imageSrc={m.poster_path || undefined}
              fallbackIcon={<RiFilmLine />}
              rating={m.rating ? String(m.rating.toFixed(1)) : undefined}
              to={`/movie/${m.id}`}
              onPlay={() => navigate(`/movie/${m.id}/watch`)}
              playNotReady={!m.file_path}
              progressRatio={progressRatio}
              completed={prog?.completed === true}
              inWatchlist={isInWatchlist('movie', m.id)}
              onWatchlistToggle={() => toggle('movie', m.id)}
              onCompletedToggle={() => handleToggleWatched(m.id)}
            />
          )
        })}
      </div>
      )}
      <Pagination page={page} totalPages={totalPages} isLoading={isLoading} onChange={goToPage} />
    </>
  )
}

// ---------- TV Shows ----------

function TVLibrary({ libraryId }: { libraryId: string }) {
  const navigate = useNavigate()
  const { isInWatchlist, toggle } = useWatchlist()
  const [loadingPlayId, setLoadingPlayId] = useState<string | null>(null)
  const [showProgressMap, setShowProgressMap] = useState<Map<string, ContinueWatchingItem>>(new Map())
  const [watchedMap, setWatchedMap] = useState<Map<string, boolean>>(new Map())
  const [sort, setSort] = useSortPref(libraryId, { field: 'title', order: 'asc' })
  const [pageSize, setPageSize] = usePageSizePref()

  useEffect(() => {
    api.getContinueWatching()
      .then(items => {
        const map = new Map<string, ContinueWatchingItem>()
        for (const item of items) {
          if (item.show_id && !map.has(item.show_id)) {
            map.set(item.show_id, item)
          }
        }
        setShowProgressMap(map)
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    api.getShowWatchStates(libraryId)
      .then(states => {
        const map = new Map<string, boolean>()
        for (const s of states) {
          map.set(s.show_id, s.total > 0 && s.completed >= s.total)
        }
        setWatchedMap(map)
      })
      .catch(() => {})
  }, [libraryId])

  const fetchPage = useCallback(
    (page: number, limit: number) => api.listTVShowsPaged({ library_id: libraryId, page, limit, sort: sort.field, order: sort.order }),
    [libraryId, sort.field, sort.order],
  )
  const { items: shows, total, page, totalPages, isLoading, error, goToPage } = usePaginatedList(fetchPage, pageSize)

  const playShow = async (showId: string) => {
    setLoadingPlayId(showId)
    try {
      const { season_id, episode_id } = await api.getNextEpisode(showId)
      navigate(`/show/${showId}/season/${season_id}/episode/${episode_id}/watch`)
    } catch {
      navigate(`/show/${showId}`)
    } finally {
      setLoadingPlayId(null)
    }
  }

  // Optimistic show-level toggle. Cascades to every episode on the server;
  // on failure we re-pull the bulk states to resync. Reading current state
  // from watchedMap directly (not from the updater) so the API call uses
  // the right value — see the matching note on handleToggleWatched in
  // MovieLibrary for the underlying React-batching reason.
  const handleToggleShowWatched = useCallback((showId: string) => {
    const next = !(watchedMap.get(showId) === true)
    setWatchedMap(prev => {
      const out = new Map(prev)
      out.set(showId, next)
      return out
    })
    // Also clear the "current episode" progress bar when unmarking, since
    // the cascade deletes every episode's progress row anyway.
    if (!next) {
      setShowProgressMap(prev => {
        if (!prev.has(showId)) return prev
        const out = new Map(prev)
        out.delete(showId)
        return out
      })
    }
    api.setShowCompleted(showId, next).catch(() => {
      api.getShowWatchStates(libraryId)
        .then(states => {
          const map = new Map<string, boolean>()
          for (const s of states) map.set(s.show_id, s.total > 0 && s.completed >= s.total)
          setWatchedMap(map)
        })
        .catch(() => {})
    })
  }, [watchedMap, libraryId])

  if (error) return <ErrorState message={error} />

  return (
    <>
      <div className={styles.toolbar}>
        <p className={`label-sm ${styles.count}`}>
          {isLoading ? 'Loading…' : formatRange(page, pageSize, shows.length, total, 'show', 'shows')}
        </p>
        <Pagination page={page} totalPages={totalPages} isLoading={isLoading} onChange={goToPage} inline />
        <div className={styles.toolbarRight}>
          <PageSizeDropdown value={pageSize} onChange={setPageSize} />
          <SortDropdown options={TV_SORT_OPTIONS} field={sort.field} order={sort.order} onChange={setSort} />
        </div>
      </div>
      {isLoading ? <LoadingGrid /> : shows.length === 0 ? <EmptyState label="No TV shows found" /> : (
      <div className={styles.grid}>
        {shows.map(s => {
          const prog = showProgressMap.get(s.id)
          const progressRatio = prog && prog.duration > 0 ? prog.position / prog.duration : undefined
          const watched = watchedMap.get(s.id) === true
          return (
            <MediaCard
              key={s.id}
              title={s.title}
              subtitle={s.year ? String(s.year) : undefined}
              imageSrc={s.poster_path || undefined}
              fallbackIcon={<RiTv2Line />}
              badge={s.status || undefined}
              to={`/show/${s.id}`}
              onPlay={() => playShow(s.id)}
              playLoading={loadingPlayId === s.id}
              progressRatio={progressRatio}
              completed={watched}
              inWatchlist={isInWatchlist('tvshow', s.id)}
              onWatchlistToggle={() => toggle('tvshow', s.id)}
              onCompletedToggle={() => handleToggleShowWatched(s.id)}
            />
          )
        })}
      </div>
      )}
      <Pagination page={page} totalPages={totalPages} isLoading={isLoading} onChange={goToPage} />
    </>
  )
}

// ---------- Music ----------

function MusicLibrary({ libraryId }: { libraryId: string }) {
  const navigate = useNavigate()
  const [artistMap, setArtistMap] = useState<Map<string, string>>(new Map())
  const [sort, setSort] = useSortPref(libraryId, { field: 'title', order: 'asc' })
  const [pageSize, setPageSize] = usePageSizePref()

  useEffect(() => {
    api.listArtists({ library_id: libraryId, limit: 200 })
      .then(artists => setArtistMap(new Map(artists.map(a => [a.id, a.name]))))
      .catch(() => {})
  }, [libraryId])

  const fetchPage = useCallback(
    (page: number, limit: number) => api.listAlbumsPaged({ library_id: libraryId, page, limit, sort: sort.field, order: sort.order }),
    [libraryId, sort.field, sort.order],
  )
  const { items: albums, total, page, totalPages, isLoading, error, goToPage } = usePaginatedList(fetchPage, pageSize)

  if (error) return <ErrorState message={error} />

  return (
    <>
      <div className={styles.toolbar}>
        <p className={`label-sm ${styles.count}`}>
          {isLoading ? 'Loading…' : formatRange(page, pageSize, albums.length, total, 'album', 'albums')}
        </p>
        <Pagination page={page} totalPages={totalPages} isLoading={isLoading} onChange={goToPage} inline />
        <div className={styles.toolbarRight}>
          <PageSizeDropdown value={pageSize} onChange={setPageSize} />
          <SortDropdown options={ALBUM_SORT_OPTIONS} field={sort.field} order={sort.order} onChange={setSort} />
        </div>
      </div>
      {isLoading ? <LoadingGrid /> : albums.length === 0 ? <EmptyState label="No albums found" /> : (
      <div className={styles.grid}>
        {albums.map(a => {
          const artistName = a.artist_id ? artistMap.get(a.artist_id) : undefined
          const subtitle = [artistName, a.year ? String(a.year) : undefined].filter(Boolean).join(' · ')
          return (
            <MediaCard
              key={a.id}
              title={a.title}
              subtitle={subtitle || undefined}
              imageSrc={a.cover_path || undefined}
              fallbackIcon={<RiMusicLine />}
              to={`/album/${a.id}`}
              onPlay={() => navigate(`/album/${a.id}/play`)}
            />
          )
        })}
      </div>
      )}
      <Pagination page={page} totalPages={totalPages} isLoading={isLoading} onChange={goToPage} />
    </>
  )
}

// ---------- Audiobooks ----------

function AudiobookLibrary({ libraryId }: { libraryId: string }) {
  const { isInWatchlist, toggle } = useWatchlist()
  const [sort, setSort] = useSortPref(libraryId, { field: 'title', order: 'asc' })
  const [pageSize, setPageSize] = usePageSizePref()
  const fetchPage = useCallback(
    (page: number, limit: number) => api.listAudiobooksPaged({ library_id: libraryId, page, limit, sort: sort.field, order: sort.order }),
    [libraryId, sort.field, sort.order],
  )
  const { items: audiobooks, total, page, totalPages, isLoading, error, goToPage } = usePaginatedList(fetchPage, pageSize)

  if (error) return <ErrorState message={error} />

  return (
    <>
      <div className={styles.toolbar}>
        <p className={`label-sm ${styles.count}`}>
          {isLoading ? 'Loading…' : formatRange(page, pageSize, audiobooks.length, total, 'audiobook', 'audiobooks')}
        </p>
        <Pagination page={page} totalPages={totalPages} isLoading={isLoading} onChange={goToPage} inline />
        <div className={styles.toolbarRight}>
          <PageSizeDropdown value={pageSize} onChange={setPageSize} />
          <SortDropdown options={AUDIOBOOK_SORT_OPTIONS} field={sort.field} order={sort.order} onChange={setSort} />
        </div>
      </div>
      {isLoading ? <LoadingGrid /> : audiobooks.length === 0 ? <EmptyState label="No audiobooks found" /> : (
      <div className={styles.grid}>
        {audiobooks.map(b => (
          <MediaCard
            key={b.id}
            title={b.title}
            subtitle={b.author || undefined}
            imageSrc={b.cover_path || undefined}
            fallbackIcon={<RiHeadphoneLine />}
            to={`/audiobook/${b.id}`}
            inWatchlist={isInWatchlist('audiobook', b.id)}
            onWatchlistToggle={() => toggle('audiobook', b.id)}
          />
        ))}
      </div>
      )}
      <Pagination page={page} totalPages={totalPages} isLoading={isLoading} onChange={goToPage} />
    </>
  )
}

// ---------- Pagination ----------

function formatRange(
  page: number,
  pageSize: number,
  resultCount: number,
  total: number,
  singular: string,
  plural: string,
): string {
  if (total === 0) return `0 ${plural}`
  const first = (page - 1) * pageSize + 1
  const last = (page - 1) * pageSize + resultCount
  const noun = total === 1 ? singular : plural
  return `Showing ${first}–${last} of ${total} ${noun}`
}

// pageWindow returns the page numbers to render in the pager, with -1
// markers where ellipses go. Always pins first and last, always shows a
// window of pages around the current one. Keeps the total button count
// bounded so the bar doesn't grow with large libraries.
function pageWindow(current: number, total: number): number[] {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1)
  }
  const out: number[] = [1]
  const start = Math.max(2, current - 1)
  const end = Math.min(total - 1, current + 1)
  if (start > 2) out.push(-1)
  for (let p = start; p <= end; p++) out.push(p)
  if (end < total - 1) out.push(-1)
  out.push(total)
  return out
}

function Pagination({ page, totalPages, isLoading, onChange, inline = false }: {
  page: number
  totalPages: number
  isLoading: boolean
  onChange: (p: number) => void
  // Inline drops the standalone vertical margin so the control sits flush in
  // the toolbar row alongside the count and sort dropdowns.
  inline?: boolean
}) {
  if (totalPages <= 1) return null
  const pages = pageWindow(page, totalPages)
  return (
    <nav className={`${styles.pagination} ${inline ? styles.paginationInline : ''}`} aria-label="Pagination">
      <button
        className={`btn btn-icon ${styles.pageBtn}`}
        onClick={() => onChange(1)}
        disabled={page <= 1 || isLoading}
        aria-label="First page"
      >
        <RiArrowLeftDoubleLine size={18} />
      </button>
      <button
        className={`btn btn-icon ${styles.pageBtn}`}
        onClick={() => onChange(page - 1)}
        disabled={page <= 1 || isLoading}
        aria-label="Previous page"
      >
        <RiArrowLeftSLine size={18} />
      </button>
      {pages.map((p, i) =>
        p === -1
          ? <span key={`gap-${i}`} className={styles.pageEllipsis}>…</span>
          : (
            <button
              key={p}
              className={`btn ${styles.pageBtn} ${p === page ? styles.pageBtnActive : ''}`}
              onClick={() => onChange(p)}
              disabled={isLoading}
              aria-current={p === page ? 'page' : undefined}
              aria-label={`Page ${p}`}
            >
              {p}
            </button>
          ),
      )}
      <button
        className={`btn btn-icon ${styles.pageBtn}`}
        onClick={() => onChange(page + 1)}
        disabled={page >= totalPages || isLoading}
        aria-label="Next page"
      >
        <RiArrowRightSLine size={18} />
      </button>
      <button
        className={`btn btn-icon ${styles.pageBtn}`}
        onClick={() => onChange(totalPages)}
        disabled={page >= totalPages || isLoading}
        aria-label="Last page"
      >
        <RiArrowRightDoubleLine size={18} />
      </button>
    </nav>
  )
}

// ---------- Shared states ----------

function LoadingGrid() {
  return (
    <div className={styles.page}>
      <div className={styles.skeletonGrid}>
        {Array.from({ length: 18 }).map((_, i) => (
          <div key={i} className={`card card-portrait ${styles.skeleton}`} />
        ))}
      </div>
    </div>
  )
}

function EmptyState({ label }: { label: string }) {
  return (
    <div className={styles.empty}>
      <p className="body-md" style={{ color: 'var(--color-on-surface-variant)', margin: 0 }}>{label}</p>
    </div>
  )
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className={styles.empty}>
      <p className="body-md" style={{ color: 'var(--color-error)', margin: 0 }}>{message}</p>
    </div>
  )
}
