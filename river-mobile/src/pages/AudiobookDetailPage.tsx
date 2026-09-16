import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { DetailHero, PlayRow } from '../components/Detail'
import { WatchlistButton } from '../components/WatchlistButton'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { formatDuration } from '../util/format'

export default function AudiobookDetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { data, loading, error, reload } = useAsync(
    async () => {
      const [book, chapters] = await Promise.all([api.getAudiobook(id), api.listChapters(id)])
      return { book, chapters: chapters.sort((a, b) => a.number - b.number) }
    },
    [id],
  )

  if (loading) return <Loading />
  if (error || !data) return <ErrorState message={error ?? 'Not found'} onRetry={reload} />

  const { book, chapters } = data
  const meta = [book.author || null, book.year > 0 ? String(book.year) : null, book.narrator ? `Narrated by ${book.narrator}` : null]
    .filter(Boolean).join('  ·  ')

  return (
    <div>
      <DetailHero
        image={book.cover_path}
        title={book.title}
        meta={meta}
        description={book.description}
        actions={<WatchlistButton mediaType="audiobook" mediaId={id} />}
      />
      {chapters.length === 0 ? <EmptyState message="No chapters." /> : chapters.map(ch => (
        <PlayRow
          key={ch.id}
          index={ch.number}
          title={ch.title || `Chapter ${ch.number}`}
          meta={ch.duration > 0 ? formatDuration(ch.duration) : undefined}
          onPlay={() => navigate(`/audiobooks/${id}/listen?chapter=${ch.id}`)}
        />
      ))}
    </div>
  )
}
