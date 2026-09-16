import { useEffect, useState } from 'react'
import { api } from '../api'
import type { SearchResult } from '../api'
import { useAsync } from '../hooks/useAsync'
import { Grid } from '../components/collections'
import { PosterCard } from '../components/PosterCard'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { imageUrl } from '../util/imageUrl'
import { initials } from '../util/format'
import { screen, heading } from './styles'

export default function SearchPage() {
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 300)
    return () => clearTimeout(t)
  }, [q])

  const { data, loading, error, reload } = useAsync<SearchResult | null>(
    async () => (debounced.length >= 2 ? api.search({ q: debounced }) : null),
    [debounced],
  )

  const hasResults = !!data && (data.libraries.some(l => l.items.length) || data.people.length)

  return (
    <div>
      <h1 style={{ ...heading, ...screen, paddingBottom: '0.75rem' }}>Search</h1>
      <div style={{ padding: '0 1.25rem 1rem' }}>
        <input
          className="input"
          value={q}
          onChange={e => setQ(e.target.value)}
          placeholder="Movies, shows, audiobooks, people…"
          autoCapitalize="none"
          autoCorrect="off"
        />
      </div>

      {debounced.length < 2 ? <EmptyState message="Type to search your library." />
        : loading ? <Loading />
        : error ? <ErrorState message={error} onRetry={reload} />
        : !hasResults ? <EmptyState message={`No results for “${debounced}”.`} />
        : (
          <div style={{ paddingBottom: '1rem' }}>
            {data!.libraries.filter(l => l.items.length > 0).map(lib => (
              <section key={lib.library_id} style={{ marginBottom: '1.25rem' }}>
                <h2 style={styles.section}>{lib.library_name}</h2>
                <Grid>
                  {lib.items.map(it => (
                    <PosterCard
                      key={it.id}
                      to={it.media_type === 'movie' ? `/movies/${it.id}` : it.media_type === 'tvshow' ? `/tvshows/${it.id}` : `/audiobooks/${it.id}`}
                      title={it.title}
                      subtitle={it.year ? String(it.year) : undefined}
                      image={it.poster_path}
                      kind={it.media_type}
                    />
                  ))}
                </Grid>
              </section>
            ))}

            {data!.people.length > 0 && (
              <section style={{ marginBottom: '1.25rem' }}>
                <h2 style={styles.section}>People</h2>
                <div style={styles.people}>
                  {data!.people.map(p => (
                    <div key={p.id} style={styles.person}>
                      <span style={styles.avatar}>
                        {imageUrl(p.profile_path) ? <img src={imageUrl(p.profile_path)} alt={p.name} style={styles.avatarImg} /> : initials(p.name)}
                      </span>
                      <span style={styles.personName}>{p.name}</span>
                    </div>
                  ))}
                </div>
              </section>
            )}
          </div>
        )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  section: { margin: '0 0 0.6rem', fontSize: '1rem', fontWeight: 700, color: 'var(--text-muted)', padding: '0 1.25rem' },
  people: { display: 'flex', gap: '1rem', overflowX: 'auto', padding: '0 1.25rem' },
  person: { flex: '0 0 5rem', width: '5rem', textAlign: 'center', color: 'inherit' },
  avatar: {
    width: '5rem', height: '5rem', borderRadius: '50%', display: 'grid', placeItems: 'center',
    background: 'var(--bg-elev-3)', fontWeight: 700, overflow: 'hidden', margin: '0 auto 0.4rem',
  },
  avatarImg: { width: '100%', height: '100%', objectFit: 'cover' },
  personName: { fontSize: '0.75rem', display: 'block', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
}
