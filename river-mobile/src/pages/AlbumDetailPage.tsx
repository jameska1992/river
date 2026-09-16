import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { DetailHero, PlayRow } from '../components/Detail'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { formatDuration } from '../util/format'

export default function AlbumDetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { data, loading, error, reload } = useAsync(
    async () => {
      const [album, tracks] = await Promise.all([api.getAlbum(id), api.listAlbumTracks(id)])
      return { album, tracks: tracks.sort((a, b) => a.number - b.number) }
    },
    [id],
  )

  if (loading) return <Loading />
  if (error || !data) return <ErrorState message={error ?? 'Not found'} onRetry={reload} />

  const { album, tracks } = data
  const meta = [album.year > 0 ? String(album.year) : null, album.genre || null, `${tracks.length} tracks`]
    .filter(Boolean).join('  ·  ')

  return (
    <div>
      <DetailHero image={album.cover_path} title={album.title} meta={meta} />
      {tracks.length === 0 ? <EmptyState message="No tracks." /> : tracks.map(t => (
        <PlayRow
          key={t.id}
          index={t.number}
          title={t.title}
          meta={t.duration > 0 ? formatDuration(t.duration) : undefined}
          onPlay={() => navigate(`/albums/${id}/play?track=${t.id}`)}
        />
      ))}
    </div>
  )
}
