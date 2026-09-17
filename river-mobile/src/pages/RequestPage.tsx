import { useCallback, useEffect, useState } from 'react'
import { RiFilmLine, RiTv2Line, RiCheckLine, RiAddLine, RiLoader4Line } from 'react-icons/ri'
import { api, ApiError } from '../api'
import type { MovieSearchResult, ShowSearchResult } from '../api'
import { imageUrl } from '../util/imageUrl'
import { requestItems, type RequestTab, type RequestItem } from '../util/requestItems'
import { Loading, EmptyState } from '../components/States'
import { heading, screen } from './styles'

type ReqState = 'idle' | 'requesting' | 'done' | 'error'

const TABS: { key: RequestTab; label: string }[] = [
  { key: 'movies', label: 'Movies' },
  { key: 'shows', label: 'TV Shows' },
]

export default function RequestPage() {
  const [tab, setTab] = useState<RequestTab>('movies')
  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')
  const [movies, setMovies] = useState<MovieSearchResult[]>([])
  const [shows, setShows] = useState<ShowSearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [notConfigured, setNotConfigured] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [reqState, setReqState] = useState<Map<number, ReqState>>(new Map())

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query.trim()), 350)
    return () => clearTimeout(t)
  }, [query])

  const search = useCallback(async (q: string, t: RequestTab) => {
    setLoading(true); setNotConfigured(false); setError(null); setReqState(new Map())
    try {
      if (t === 'movies') { setMovies(await api.searchMovieRequests(q)); setShows([]) }
      else { setShows(await api.searchShowRequests(q)); setMovies([]) }
    } catch (e) {
      // 503 = the Radarr/Sonarr integration isn't configured on the server.
      if (e instanceof ApiError && e.status === 503) setNotConfigured(true)
      else setError(e instanceof Error ? e.message : 'Search failed')
      setMovies([]); setShows([])
    } finally {
      setLoading(false)
    }
  }, [])

  // Run (or re-run on tab change) whenever there's a query. When it's empty we
  // simply render the prompt below and ignore any stale results.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- kicks off the debounced search; the setState happens inside the async callback
    if (debounced) void search(debounced, tab)
  }, [debounced, tab, search])

  const request = useCallback(async (item: RequestItem) => {
    setReqState(prev => new Map(prev).set(item.id, 'requesting'))
    try {
      if (item.kind === 'movies') await api.requestMovie(item.id, item.title, item.year)
      else await api.requestShow(item.id, item.title, item.year)
      setReqState(prev => new Map(prev).set(item.id, 'done'))
    } catch {
      setReqState(prev => new Map(prev).set(item.id, 'error'))
    }
  }, [])

  const items = requestItems(tab, movies, shows)
  const integration = tab === 'movies' ? 'Radarr' : 'Sonarr'

  return (
    <div>
      <h1 style={{ ...heading, ...screen, paddingBottom: '0.75rem' }}>Request</h1>

      <div style={styles.tabs}>
        {TABS.map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            style={{ ...styles.tab, ...(tab === t.key ? styles.tabActive : {}) }}
          >
            {t.key === 'movies' ? <RiFilmLine /> : <RiTv2Line />} {t.label}
          </button>
        ))}
      </div>

      <div style={{ padding: '0 1.25rem 1rem' }}>
        <input
          className="input"
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder={`Search ${tab === 'movies' ? 'movies' : 'TV shows'} to request…`}
          autoCapitalize="none"
          autoCorrect="off"
        />
      </div>

      {debounced.length === 0 ? <EmptyState message="Search to request something new." />
        : loading ? <Loading />
        : notConfigured ? <EmptyState message={`${integration} isn't configured on the server.`} />
        : error ? <EmptyState message={error} />
        : items.length === 0 ? <EmptyState message={`No results for “${debounced}”.`} />
        : (
          <div style={styles.list}>
            {items.map(item => {
              const state = reqState.get(item.id) ?? 'idle'
              const added = item.added || state === 'done'
              const poster = imageUrl(item.poster, 'poster')
              return (
                <div key={item.id} style={styles.card}>
                  <div style={styles.poster}>
                    {poster
                      ? <img src={poster} alt="" loading="lazy" style={styles.posterImg} />
                      : <span style={styles.posterFallback}>{item.kind === 'movies' ? <RiFilmLine /> : <RiTv2Line />}</span>}
                  </div>
                  <div style={styles.info}>
                    <div style={styles.title}>{item.title}</div>
                    <div style={styles.year}>{item.year > 0 ? item.year : '—'}</div>
                    {item.overview && <div style={styles.overview}>{item.overview}</div>}
                    <button
                      className={`btn ${added ? '' : 'btn-primary'}`}
                      style={styles.reqBtn}
                      disabled={added || state === 'requesting'}
                      onClick={() => void request(item)}
                    >
                      {state === 'requesting' ? <RiLoader4Line /> : added ? <RiCheckLine /> : <RiAddLine />}
                      {state === 'requesting' ? 'Requesting…' : added ? 'Requested' : state === 'error' ? 'Retry' : 'Request'}
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  tabs: { display: 'flex', gap: '0.5rem', padding: '0 1.25rem 1rem', overflowX: 'auto' },
  tab: {
    display: 'flex', alignItems: 'center', gap: '0.4rem',
    padding: '0.5rem 1rem', borderRadius: '999px', background: 'var(--bg-elev-2)',
    color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.9rem', whiteSpace: 'nowrap',
  },
  tabActive: { background: 'var(--accent)', color: 'var(--on-accent)' },
  list: { display: 'flex', flexDirection: 'column', gap: '1rem', padding: '0 1.25rem 1rem' },
  card: { display: 'flex', gap: '0.9rem' },
  poster: {
    flex: '0 0 5rem', width: '5rem', aspectRatio: '2 / 3', borderRadius: 'var(--radius-md)',
    overflow: 'hidden', background: 'var(--bg-elev-2)',
  },
  posterImg: { width: '100%', height: '100%', objectFit: 'cover', display: 'block' },
  posterFallback: {
    width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center',
    color: 'var(--text-muted)', fontSize: '1.5rem',
  },
  info: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: '0.15rem' },
  title: { fontWeight: 700 },
  year: { fontSize: '0.8rem', color: 'var(--text-muted)' },
  overview: {
    fontSize: '0.8rem', color: 'var(--text-muted)', lineHeight: 1.4, margin: '0.25rem 0 0.5rem',
    display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical', overflow: 'hidden',
  },
  reqBtn: { alignSelf: 'flex-start', marginTop: 'auto', gap: '0.4rem', fontSize: '0.85rem', padding: '0.45rem 0.9rem' },
}
