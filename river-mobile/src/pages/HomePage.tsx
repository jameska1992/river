import { api } from '../api'
import type { ContinueWatchingItem, NextUpItem, RecentlyAddedItem } from '../api'
import { useAsync } from '../hooks/useAsync'
import { Row, RailItem } from '../components/collections'
import { PosterCard } from '../components/PosterCard'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { detailPath, episodeCode } from '../util/format'
import { heading, screen } from './styles'

interface HomeData {
  continueWatching: ContinueWatchingItem[]
  nextUp: NextUpItem[]
  recent: RecentlyAddedItem[]
}

// Where a continue-watching row points — the parent detail page for its type
// (the player itself lands in #143/#144; browse opens the detail).
function cwPath(item: ContinueWatchingItem): string {
  if (item.media_type === 'movie') return detailPath('movie', item.media_id)
  if (item.media_type === 'chapter' && item.audiobook_id) return detailPath('audiobook', item.audiobook_id)
  if (item.show_id) return detailPath('tvshow', item.show_id)
  return '/'
}

export default function HomePage() {
  const { data, loading, error, reload } = useAsync<HomeData>(async () => {
    const [continueWatching, nextUp, recent] = await Promise.all([
      api.getContinueWatching().catch(() => []),
      api.getNextUp().catch(() => []),
      api.getRecentlyAdded().catch(() => []),
    ])
    return { continueWatching, nextUp, recent }
  }, [])

  if (loading) return <Loading />
  if (error) return <ErrorState message={error} onRetry={reload} />

  const d = data!
  const anything = d.continueWatching.length || d.nextUp.length || d.recent.length
  if (!anything) return <EmptyState message="Nothing here yet. Add media on the server and it'll show up." />

  return (
    <div style={{ paddingTop: '1rem' }}>
      <h1 style={{ ...heading, ...screen, paddingBottom: 0 }}>Home</h1>

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

      {d.recent.length > 0 && (
        <Row title="Recently Added">
          {d.recent.map(item => (
            <RailItem key={`${item.media_type}-${item.id}`}>
              <PosterCard
                to={detailPath(item.media_type, item.id)}
                title={item.title}
                subtitle={item.year > 0 ? String(item.year) : undefined}
                image={item.poster_path}
                kind={item.media_type}
              />
            </RailItem>
          ))}
        </Row>
      )}
    </div>
  )
}
