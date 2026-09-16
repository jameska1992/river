import { useState } from 'react'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { Grid } from '../components/collections'
import { PosterCard, type PosterCardProps } from '../components/PosterCard'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { heading, screen } from './styles'

type Tab = 'movies' | 'tvshows' | 'music' | 'audiobooks'
const TABS: { key: Tab; label: string }[] = [
  { key: 'movies', label: 'Movies' },
  { key: 'tvshows', label: 'TV' },
  { key: 'music', label: 'Music' },
  { key: 'audiobooks', label: 'Books' },
]

// Fetch the selected type and normalise each into PosterCard props.
async function fetchTab(tab: Tab): Promise<PosterCardProps[]> {
  switch (tab) {
    case 'movies': {
      const items = await api.listMovies({ limit: 60 })
      return items.map(m => ({ to: `/movies/${m.id}`, title: m.title, subtitle: m.year ? String(m.year) : undefined, image: m.poster_path, kind: 'movie' }))
    }
    case 'tvshows': {
      const items = await api.listTVShows({ limit: 60 })
      return items.map(s => ({ to: `/tvshows/${s.id}`, title: s.title, subtitle: s.year ? String(s.year) : undefined, image: s.poster_path, kind: 'tvshow' }))
    }
    case 'music': {
      const items = await api.listAlbums({ limit: 60 })
      return items.map(a => ({ to: `/albums/${a.id}`, title: a.title, subtitle: a.year ? String(a.year) : undefined, image: a.cover_path, kind: 'album' }))
    }
    case 'audiobooks': {
      const items = await api.listAudiobooks({ limit: 60 })
      return items.map(b => ({ to: `/audiobooks/${b.id}`, title: b.title, subtitle: b.author || undefined, image: b.cover_path, kind: 'audiobook' }))
    }
  }
}

export default function LibraryPage() {
  const [tab, setTab] = useState<Tab>('movies')
  const { data, loading, error, reload } = useAsync(() => fetchTab(tab), [tab])

  return (
    <div>
      <h1 style={{ ...heading, ...screen, paddingBottom: '0.75rem' }}>Library</h1>

      <div style={styles.tabs}>
        {TABS.map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            style={{ ...styles.tab, ...(tab === t.key ? styles.tabActive : {}) }}
          >
            {t.label}
          </button>
        ))}
      </div>

      {loading ? <Loading />
        : error ? <ErrorState message={error} onRetry={reload} />
        : !data || data.length === 0 ? <EmptyState message="Nothing in this library yet." />
        : <Grid>{data.map(c => <PosterCard key={c.to} {...c} />)}</Grid>}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  tabs: { display: 'flex', gap: '0.5rem', padding: '0 1.25rem 1rem', overflowX: 'auto' },
  tab: {
    padding: '0.5rem 1rem', borderRadius: '999px', background: 'var(--bg-elev-2)',
    color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.9rem', whiteSpace: 'nowrap',
  },
  tabActive: { background: 'var(--accent)', color: 'var(--on-accent)' },
}
