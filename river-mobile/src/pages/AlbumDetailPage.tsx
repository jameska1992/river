import { useParams } from 'react-router-dom'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { useAudioPlayer } from '../context/audioPlayerContext'
import { trackToAudioItem } from '../util/audio'
import { DetailHero, PlayRow } from '../components/Detail'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { formatDuration } from '../util/format'

export default function AlbumDetailPage() {
  const { id = '' } = useParams()
  const { playQueue } = useAudioPlayer()
  const { data, loading, error, reload } = useAsync(
    async () => {
      const [album, tracks] = await Promise.all([api.getAlbum(id), api.listAlbumTracks(id)])
      // Artist name is a separate record; best-effort for display + metadata.
      let artistName = ''
      try { artistName = (await api.getArtist(album.artist_id)).name } catch { /* optional */ }
      return { album, artistName, tracks: tracks.sort((a, b) => a.number - b.number) }
    },
    [id],
  )

  if (loading) return <Loading />
  if (error || !data) return <ErrorState message={error ?? 'Not found'} onRetry={reload} />

  const { album, artistName, tracks } = data
  const meta = [artistName || null, album.year > 0 ? String(album.year) : null, album.genre || null, `${tracks.length} tracks`]
    .filter(Boolean).join('  ·  ')

  // Playable queue (tracks with a file), in display order.
  const items = tracks.filter(t => t.file_path).map(t => trackToAudioItem(t, album, artistName))
  const play = (trackId: string) => {
    const start = items.findIndex(it => it.id === trackId)
    playQueue(items, start < 0 ? 0 : start)
  }

  return (
    <div>
      <DetailHero image={album.cover_path} title={album.title} meta={meta} />
      {tracks.length === 0 ? <EmptyState message="No tracks." /> : tracks.map(t => (
        <PlayRow
          key={t.id}
          index={t.number}
          title={t.title}
          meta={t.duration > 0 ? formatDuration(t.duration) : undefined}
          onPlay={() => play(t.id)}
        />
      ))}
    </div>
  )
}
