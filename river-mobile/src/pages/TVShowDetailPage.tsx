import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api'
import type { Season } from '../api'
import { useAsync } from '../hooks/useAsync'
import { DetailHero, PlayRow } from '../components/Detail'
import { PeopleRow } from '../components/People'
import { WatchlistButton } from '../components/WatchlistButton'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { episodeCode } from '../util/format'
import { castToEntries, crewToEntries } from '../util/credits'

export default function TVShowDetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { data, loading, error, reload } = useAsync(
    async () => {
      // Credits are best-effort so an un-enriched show still renders.
      const [show, seasons, credits] = await Promise.all([
        api.getTVShow(id),
        api.listSeasons(id),
        api.getTVShowCredits(id).catch(() => null),
      ])
      return { show, seasons: seasons.sort((a, b) => a.number - b.number), credits }
    },
    [id],
  )

  // seasonId is null until the user picks one; activeSeason falls back to the
  // first season, so episodes load for it from the first render.
  const [seasonId, setSeasonId] = useState<string | null>(null)
  const activeSeason: Season | undefined = data?.seasons.find(s => s.id === seasonId) ?? data?.seasons[0]

  const episodes = useAsync(
    async () => (activeSeason ? api.listEpisodes(id, activeSeason.id) : []),
    [id, activeSeason?.id],
  )

  if (loading) return <Loading />
  if (error || !data) return <ErrorState message={error ?? 'Not found'} onRetry={reload} />

  const { show, seasons, credits } = data
  const meta = [show.year > 0 ? String(show.year) : null, show.status || null, ...(show.genres ?? [])]
    .filter(Boolean).join('  ·  ')

  return (
    <div>
      <DetailHero
        image={show.backdrop_path || show.poster_path}
        landscape={!!show.backdrop_path}
        title={show.title}
        meta={meta}
        description={show.description}
        actions={<WatchlistButton mediaType="tvshow" mediaId={id} />}
      />

      {seasons.length === 0 ? <EmptyState message="No seasons yet." /> : (
        <>
          <div style={styles.seasonTabs}>
            {seasons.map(s => (
              <button
                key={s.id}
                onClick={() => setSeasonId(s.id)}
                style={{ ...styles.seasonTab, ...(activeSeason?.id === s.id ? styles.seasonTabActive : {}) }}
              >
                {s.number === 0 ? 'Specials' : `Season ${s.number}`}
              </button>
            ))}
          </div>

          {episodes.loading ? <Loading />
            : episodes.error ? <ErrorState message={episodes.error} onRetry={episodes.reload} />
            : !episodes.data || episodes.data.length === 0 ? <EmptyState message="No episodes." />
            : episodes.data.map(ep => (
              <PlayRow
                key={ep.id}
                index={ep.is_special ? 'SP' : ep.number}
                title={ep.title || episodeCode(activeSeason?.number ?? 0, ep.number)}
                meta={ep.runtime > 0 ? `${ep.runtime}m` : undefined}
                onPlay={() => navigate(`/tvshows/${id}/seasons/${ep.season_id}/episodes/${ep.id}/watch`)}
              />
            ))}
        </>
      )}

      {credits && <PeopleRow title="Cast" people={castToEntries(credits.cast)} />}
      {credits && <PeopleRow title="Crew" people={crewToEntries(credits.crew)} />}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  seasonTabs: { display: 'flex', gap: '0.5rem', padding: '0 1.25rem 0.75rem', overflowX: 'auto' },
  seasonTab: {
    padding: '0.4rem 0.9rem', borderRadius: '999px', background: 'var(--bg-elev-2)',
    color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.85rem', whiteSpace: 'nowrap',
  },
  seasonTabActive: { background: 'var(--accent)', color: 'var(--on-accent)' },
}
