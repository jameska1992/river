import { Link } from 'react-router-dom'
import { RiStackLine } from 'react-icons/ri'
import { api } from '../api'
import type { ContinueWatchingItem, NextUpItem, WatchlistItem, Collection, Library } from '../api'
import { useAsync } from '../hooks/useAsync'
import { Row, RailItem } from '../components/collections'
import { PosterCard, type PosterCardProps } from '../components/PosterCard'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { imageUrl } from '../util/imageUrl'
import { detailPath, episodeCode } from '../util/format'
import { heading, screen } from './styles'

interface LibrarySection {
  id: string
  name: string
  items: PosterCardProps[]
}

interface HomeData {
  continueWatching: ContinueWatchingItem[]
  nextUp: NextUpItem[]
  watchlist: WatchlistItem[]
  collections: Collection[]
  libraries: LibrarySection[]
}

// Where a continue-watching row points — the parent detail page for its type.
function cwPath(item: ContinueWatchingItem): string {
  if (item.media_type === 'movie') return detailPath('movie', item.media_id)
  if (item.media_type === 'chapter' && item.audiobook_id) return detailPath('audiobook', item.audiobook_id)
  if (item.show_id) return detailPath('tvshow', item.show_id)
  return '/'
}

// Recently-added items for one library, as PosterCard props (matches
// river-web's per-library carousels).
async function libraryRow(lib: Library): Promise<PosterCardProps[]> {
  const p = { library_id: lib.id, limit: 20, sort: 'added', order: 'desc' as const }
  switch (lib.type) {
    case 'movie': {
      const items = await api.listMovies(p)
      return items.map(m => ({ to: detailPath('movie', m.id), title: m.title, subtitle: m.year > 0 ? String(m.year) : undefined, image: m.poster_path, kind: 'movie' }))
    }
    case 'tvshow': {
      const items = await api.listTVShows(p)
      return items.map(s => ({ to: detailPath('tvshow', s.id), title: s.title, subtitle: s.year > 0 ? String(s.year) : undefined, image: s.poster_path, kind: 'tvshow' }))
    }
    case 'music': {
      const items = await api.listAlbums(p)
      return items.map(a => ({ to: detailPath('album', a.id), title: a.title, subtitle: a.year > 0 ? String(a.year) : undefined, image: a.cover_path, kind: 'album' }))
    }
    case 'audiobook': {
      const items = await api.listAudiobooks(p)
      return items.map(b => ({ to: detailPath('audiobook', b.id), title: b.title, subtitle: b.author || undefined, image: b.cover_path, kind: 'audiobook' }))
    }
  }
}

export default function HomePage() {
  const { data, loading, error, reload } = useAsync<HomeData>(async () => {
    const [continueWatching, nextUp, watchlist, collections, libs] = await Promise.all([
      api.getContinueWatching().catch(() => []),
      api.getNextUp().catch(() => []),
      api.getWatchlist().catch(() => []),
      api.listCollections().catch(() => []),
      api.listLibraries().catch(() => []),
    ])
    const libraries = await Promise.all(
      libs.map(async lib => ({ id: lib.id, name: lib.name, items: await libraryRow(lib).catch(() => []) })),
    )
    return { continueWatching, nextUp, watchlist, collections, libraries }
  }, [])

  if (loading) return <Loading />
  if (error) return <ErrorState message={error} onRetry={reload} />

  const d = data!
  const anything =
    d.continueWatching.length || d.nextUp.length || d.watchlist.length ||
    d.collections.length || d.libraries.some(l => l.items.length)
  if (!anything) return <EmptyState message="Nothing here yet. Add media on the server and it'll show up." />

  return (
    <div style={{ paddingTop: '1rem' }}>
      <h1 style={{ ...heading, ...screen, paddingBottom: 0 }}>Home</h1>

      {d.nextUp.length > 0 && (
        <Row title="Next Up">
          {d.nextUp.map(item => (
            <RailItem key={item.media_id} width="13rem">
              <PosterCard
                to={detailPath('tvshow', item.show_id)}
                title={`${episodeCode(item.season_number, item.episode_number)} · ${item.title}`}
                subtitle={item.show_title}
                image={item.backdrop_path || item.poster_path}
                kind="episode"
                landscape
              />
            </RailItem>
          ))}
        </Row>
      )}

      {d.continueWatching.length > 0 && (
        <Row title="Continue Watching">
          {d.continueWatching.map(item => (
            <RailItem key={item.media_id} width="13rem">
              <PosterCard
                to={cwPath(item)}
                title={item.title}
                subtitle={item.show_title}
                image={item.backdrop_path || item.poster_path}
                kind={item.media_type === 'movie' ? 'movie' : item.media_type === 'chapter' ? 'chapter' : 'episode'}
                landscape
                position={item.position}
                duration={item.duration}
              />
            </RailItem>
          ))}
        </Row>
      )}

      {d.watchlist.length > 0 && (
        <Row title="Watchlist">
          {d.watchlist.map(item => (
            <RailItem key={`${item.media_type}-${item.media_id}`}>
              <PosterCard
                to={detailPath(item.media_type, item.media_id)}
                title={item.title}
                subtitle={item.year > 0 ? String(item.year) : undefined}
                image={item.poster_path}
                kind={item.media_type}
              />
            </RailItem>
          ))}
        </Row>
      )}

      {d.collections.length > 0 && (
        <Row title="Collections">
          {d.collections.map(c => (
            <RailItem key={c.id}>
              <CollectionTile collection={c} />
            </RailItem>
          ))}
        </Row>
      )}

      {d.libraries.filter(l => l.items.length > 0).map(lib => (
        <Row key={lib.id} title={lib.name}>
          {lib.items.map(card => (
            <RailItem key={card.to}>
              <PosterCard {...card} />
            </RailItem>
          ))}
        </Row>
      ))}
    </div>
  )
}

function CollectionTile({ collection }: { collection: Collection }) {
  const cover = imageUrl(collection.covers?.[0], 'poster')
  return (
    <Link to={`/collections/${collection.id}`} style={styles.tile}>
      <div style={styles.tileArt}>
        {cover
          ? <img src={cover} alt="" loading="lazy" style={styles.tileImg} />
          : <div style={styles.tileFallback}><RiStackLine /></div>}
      </div>
      <div style={styles.tileTitle}>{collection.name}</div>
      <div style={styles.tileSub}>{collection.item_count} {collection.item_count === 1 ? 'item' : 'items'}</div>
    </Link>
  )
}

const styles: Record<string, React.CSSProperties> = {
  tile: { display: 'block', color: 'inherit' },
  tileArt: {
    position: 'relative', width: '100%', aspectRatio: '2 / 3',
    borderRadius: 'var(--radius-md)', overflow: 'hidden', background: 'var(--bg-elev-2)',
  },
  tileImg: { width: '100%', height: '100%', objectFit: 'cover', display: 'block' },
  tileFallback: {
    width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center',
    color: 'var(--text-muted)', fontSize: '2rem',
  },
  tileTitle: {
    marginTop: '0.4rem', fontSize: '0.85rem', fontWeight: 600,
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  tileSub: { fontSize: '0.75rem', color: 'var(--text-muted)' },
}
