import { useMemo, useState } from 'react'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { Grid } from '../components/collections'
import { PosterCard, type PosterCardProps } from '../components/PosterCard'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { libraryTabs, TAB_LABEL, type LibraryTab as Tab } from '../util/libraryTabs'
import { heading, screen } from './styles'

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
  // Tabs are driven by what libraries actually exist — no point offering a
  // Music tab when no music library is configured.
  const libraries = useAsync(() => api.listLibraries(), [])
  const tabs = useMemo<Tab[]>(
    () => (libraries.data ? libraryTabs(libraries.data.map(l => l.type)) : []),
    [libraries.data],
  )

  // Selected tab is null until the user picks; fall back to the first available
  // (and never leave a selection that's no longer offered).
  const [picked, setPicked] = useState<Tab | null>(null)
  const activeTab: Tab | null = (picked && tabs.includes(picked) ? picked : tabs[0]) ?? null

  const content = useAsync(() => (activeTab ? fetchTab(activeTab) : Promise.resolve([])), [activeTab])

  if (libraries.loading) return <><Header /><Loading /></>
  if (libraries.error) return <><Header /><ErrorState message={libraries.error} onRetry={libraries.reload} /></>
  if (tabs.length === 0) return <><Header /><EmptyState message="No libraries configured yet." /></>

  return (
    <div>
      <Header />
      <div style={styles.tabs}>
        {tabs.map(t => (
          <button
            key={t}
            onClick={() => setPicked(t)}
            style={{ ...styles.tab, ...(activeTab === t ? styles.tabActive : {}) }}
          >
            {TAB_LABEL[t]}
          </button>
        ))}
      </div>

      {content.loading ? <Loading />
        : content.error ? <ErrorState message={content.error} onRetry={content.reload} />
        : !content.data || content.data.length === 0 ? <EmptyState message="Nothing in this library yet." />
        : <Grid>{content.data.map(c => <PosterCard key={c.to} {...c} />)}</Grid>}
    </div>
  )
}

function Header() {
  return <h1 style={{ ...heading, ...screen, paddingBottom: '0.75rem' }}>Library</h1>
}

const styles: Record<string, React.CSSProperties> = {
  tabs: { display: 'flex', gap: '0.5rem', padding: '0 1.25rem 1rem', overflowX: 'auto' },
  tab: {
    padding: '0.5rem 1rem', borderRadius: '999px', background: 'var(--bg-elev-2)',
    color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.9rem', whiteSpace: 'nowrap',
  },
  tabActive: { background: 'var(--accent)', color: 'var(--on-accent)' },
}
