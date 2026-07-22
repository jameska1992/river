import { useEffect, useRef, useState, type ChangeEvent } from 'react'
// keep useEffect for the debounced query, even though the input
// observer is gone.
import { useNavigate } from 'react-router-dom'
import { api, type SearchResult } from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'
import { Sidebar } from '../components/Sidebar'
import { Row } from '../components/Row'
import { PosterCard } from '../components/PosterCard'
import { CastCard } from '../components/CastCard'

// Wait this long after the last keystroke before firing the query.
const DEBOUNCE_MS = 300

export default function SearchPage() {
  const navigate = useNavigate()
  const [q, setQ] = useState('')
  const [result, setResult] = useState<SearchResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Debounce + race-protect: only commit the most recent in-flight
  // response. Without the alive flag a slow request landing after a
  // fast one would overwrite newer results.
  useEffect(() => {
    if (!q.trim()) {
      setResult(null)
      setLoading(false)
      return
    }
    let alive = true
    setLoading(true)
    const t = window.setTimeout(() => {
      api.search({ q })
        .then(r => { if (alive) { setResult(r); setError(null) } })
        .catch(e => { if (alive) setError(String(e?.message ?? e)) })
        .finally(() => { if (alive) setLoading(false) })
    }, DEBOUNCE_MS)
    return () => { alive = false; window.clearTimeout(t) }
  }, [q])

  const hasAnyResults = !!result && (
    result.libraries.some(l => l.items.length > 0) ||
    result.people.length > 0
  )

  return (
    <FocusProvider onBack={() => navigate('/')}>
      <div style={styles.page}>
        <Sidebar />

        <main style={styles.main}>
          <div style={styles.header}>
            <h1 style={styles.title}>Search</h1>
            <SearchInput
              value={q}
              onChange={setQ}
            />
          </div>

          {error && <div style={styles.error}>{error}</div>}

          {q.trim() && !loading && !hasAnyResults && !error && (
            <p style={styles.empty}>No results for "{q.trim()}".</p>
          )}

          {result && (
            <div style={styles.sections}>
              {result.people.length > 0 && (
                <Row title="People">
                  {result.people.map(p => (
                    <CastCard
                      key={p.id}
                      name={p.name}
                      photoUrl={p.profile_path || undefined}
                      onSelect={() => navigate(`/people/${p.id}`)}
                    />
                  ))}
                </Row>
              )}

              {result.libraries.map(lib => (
                lib.items.length > 0 && (
                  <Row key={lib.library_id} title={lib.library_name}>
                    {lib.items.map(item => (
                      <PosterCard
                        key={`${item.media_type}:${item.id}`}
                        title={item.title}
                        subtitle={item.year ? String(item.year) : undefined}
                        imageSrc={item.poster_path || undefined}
                        onSelect={() => {
                          if (item.media_type === 'movie') navigate(`/movies/${item.id}`)
                          else if (item.media_type === 'tvshow') navigate(`/tvshows/${item.id}`)
                          // audiobook detail page not yet built — no-op.
                        }}
                      />
                    ))}
                  </Row>
                )
              ))}
            </div>
          )}
        </main>
      </div>
    </FocusProvider>
  )
}

function SearchInput({
  value, onChange,
}: {
  value: string
  onChange: (next: string) => void
}) {
  // Wrapper takes focus from the spatial manager; DOM focus (and
  // therefore the system keyboard) only fires when the user presses
  // OK on the focused wrapper. Escape blurs the input so arrow-key
  // nav takes over again.
  const inputRef = useRef<HTMLInputElement>(null)
  const wrapRef = useFocusable<HTMLDivElement>(
    () => inputRef.current?.focus(),
    { autoFocus: true, overrides: { left: 'sidebar' } },
  )

  return (
    <div ref={wrapRef} tabIndex={-1} style={styles.searchWrap}>
      <span style={styles.searchIcon}>⌕</span>
      <input
        ref={inputRef}
        type="search"
        value={value}
        onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        onKeyDown={e => {
          if (e.key === 'Escape' || e.key === 'GoBack') {
            e.stopPropagation()
            inputRef.current?.blur()
          }
        }}
        placeholder="Search movies, shows, people…"
        style={styles.searchInput}
        autoComplete="off"
        autoCorrect="off"
      />
    </div>
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
  header: {
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
  },
  title: {
    margin: 0,
    fontSize: '2rem',
    fontWeight: 700,
    letterSpacing: '-0.02em',
  },
  searchWrap: {
    display: 'flex',
    alignItems: 'center',
    gap: '1rem',
    background: 'var(--bg-elev)',
    borderRadius: 'var(--radius-md)',
    padding: '0 1.25rem',
    maxWidth: '50rem',
    scrollMargin: '2rem',
  },
  searchIcon: {
    fontSize: '1.5rem',
    color: 'var(--text-muted)',
  },
  searchInput: {
    flex: 1,
    minWidth: 0,
    height: '3.5rem',
    background: 'transparent',
    border: 'none',
    outline: 'none',
    color: 'var(--text)',
    fontSize: '1.15rem',
    fontFamily: 'inherit',
  },
  sections: {
    display: 'flex',
    flexDirection: 'column',
    gap: '1.5rem',
  },
  empty: {
    color: 'var(--text-muted)',
    fontSize: '1rem',
  },
  error: {
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}
