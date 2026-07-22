import { useCallback, useEffect, useRef, useState } from 'react'
import { RiFilmLine, RiTv2Line, RiCheckLine, RiAddLine, RiLoaderLine } from 'react-icons/ri'
import { api, ApiError } from '../api'
import type { MovieSearchResult, ShowSearchResult } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './RequestPage.module.css'

type Tab = 'movies' | 'shows'
type RequestState = 'idle' | 'requesting' | 'done' | 'error'

export function RequestPage() {
  const [tab, setTab] = useState<Tab>('movies')
  const [query, setQuery] = useState('')
  const [movieResults, setMovieResults] = useState<MovieSearchResult[]>([])
  const [showResults, setShowResults]   = useState<ShowSearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [notConfigured, setNotConfigured] = useState(false)
  const [searchError, setSearchError] = useState<string | null>(null)

  const [reqStates, setReqStates]   = useState<Map<number, RequestState>>(new Map())
  const [reqErrors, setReqErrors]   = useState<Map<number, string>>(new Map())

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const runSearch = useCallback(async (q: string, t: Tab) => {
    setLoading(true)
    setNotConfigured(false)
    setSearchError(null)
    setReqStates(new Map())
    setReqErrors(new Map())
    try {
      if (t === 'movies') {
        setMovieResults(await api.searchMovieRequests(q))
        setShowResults([])
      } else {
        setShowResults(await api.searchShowRequests(q))
        setMovieResults([])
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 503) {
        setNotConfigured(true)
      } else {
        setSearchError(err instanceof Error ? err.message : 'Search failed')
      }
      setMovieResults([])
      setShowResults([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (!query.trim()) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- debounced search: clears results when the query is emptied
      setMovieResults([])
      setShowResults([])
      setNotConfigured(false)
      return
    }
    debounceRef.current = setTimeout(() => {
      void runSearch(query.trim(), tab)
    }, 400)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [query, tab, runSearch])

  function switchTab(t: Tab) {
    setTab(t)
    setMovieResults([])
    setShowResults([])
    setNotConfigured(false)
    setSearchError(null)
  }

  async function handleRequest(id: number, item: MovieSearchResult | ShowSearchResult) {
    setReqStates(prev => new Map(prev).set(id, 'requesting'))
    try {
      if (tab === 'movies') {
        const m = item as MovieSearchResult
        await api.requestMovie(m.tmdbId, m.title, m.year)
      } else {
        const s = item as ShowSearchResult
        await api.requestShow(s.tvdbId, s.title, s.year)
      }
      setReqStates(prev => new Map(prev).set(id, 'done'))
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Request failed'
      setReqStates(prev => new Map(prev).set(id, 'error'))
      setReqErrors(prev => new Map(prev).set(id, msg))
    }
  }

  const results = tab === 'movies'
    ? movieResults.map(r => ({ id: r.tmdbId, item: r }))
    : showResults.map(r => ({ id: r.tvdbId, item: r }))

  return (
    <div className="container" style={{ paddingTop: 'var(--space-5)' }}>
      <h1 className={`headline-sm ${styles.heading}`}>Request Media</h1>

      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${tab === 'movies' ? styles.tabActive : ''}`}
          onClick={() => switchTab('movies')}
        >
          <RiFilmLine /> Movies
        </button>
        <button
          className={`${styles.tab} ${tab === 'shows' ? styles.tabActive : ''}`}
          onClick={() => switchTab('shows')}
        >
          <RiTv2Line /> TV Shows
        </button>
      </div>

      <input
        className={`input ${styles.searchInput}`}
        type="search"
        placeholder={`Search ${tab === 'movies' ? 'movies' : 'TV shows'}…`}
        value={query}
        onChange={e => setQuery(e.target.value)}
        autoFocus
      />

      {loading && (
        <div className={styles.status}>
          <RiLoaderLine className={styles.spinner} /> Searching…
        </div>
      )}

      {notConfigured && !loading && (
        <div className={styles.status}>
          {tab === 'movies' ? 'Radarr' : 'Sonarr'} is not configured.
          Set <code>{tab === 'movies' ? 'RADARR_URL / RADARR_API_KEY' : 'SONARR_URL / SONARR_API_KEY'}</code> in river-api.
        </div>
      )}

      {searchError && !loading && (
        <div className={`${styles.status} ${styles.statusError}`}>{searchError}</div>
      )}

      {!loading && !notConfigured && !searchError && query.trim() && results.length === 0 && (
        <div className={styles.status}>No results found.</div>
      )}

      <div className={styles.results}>
        {results.map(({ id, item }) => {
          const state  = reqStates.get(id) ?? 'idle'
          const error  = reqErrors.get(id)
          const added  = item.added || state === 'done'
          return (
            <div key={id} className={styles.card}>
              <div className={styles.poster}>
                {item.poster
                  ? <img src={imageUrl(item.poster)} alt={item.title} />
                  : <div className={styles.posterFallback}>{tab === 'movies' ? <RiFilmLine /> : <RiTv2Line />}</div>
                }
              </div>
              <div className={styles.info}>
                <p className={`label-md ${styles.title}`}>{item.title}</p>
                <p className={`label-sm ${styles.year}`}>{item.year || '—'}</p>
                <p className={`body-sm ${styles.overview}`}>{item.overview}</p>
                <div className={styles.cardFooter}>
                  {error && <span className={`label-sm ${styles.errorText}`}>{error}</span>}
                  <button
                    className={`btn btn-sm ${added ? styles.btnAdded : 'btn-primary'}`}
                    disabled={added || state === 'requesting'}
                    onClick={() => void handleRequest(id, item)}
                  >
                    {state === 'requesting' && <RiLoaderLine className={styles.spinner} />}
                    {added && <RiCheckLine />}
                    {!added && state !== 'requesting' && <RiAddLine />}
                    {state === 'requesting' ? 'Requesting…' : added ? 'Requested' : 'Request'}
                  </button>
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
