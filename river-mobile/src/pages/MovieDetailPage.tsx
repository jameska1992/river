import { useParams, useNavigate } from 'react-router-dom'
import { RiPlayFill } from 'react-icons/ri'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { DetailHero } from '../components/Detail'
import { WatchlistButton } from '../components/WatchlistButton'
import { Loading, ErrorState } from '../components/States'

export default function MovieDetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { data: movie, loading, error, reload } = useAsync(() => api.getMovie(id), [id])

  if (loading) return <Loading />
  if (error || !movie) return <ErrorState message={error ?? 'Not found'} onRetry={reload} />

  const meta = [
    movie.year > 0 ? String(movie.year) : null,
    movie.runtime > 0 ? `${movie.runtime} min` : null,
    movie.rating > 0 ? `★ ${movie.rating.toFixed(1)}` : null,
    ...(movie.genres ?? []),
  ].filter(Boolean).join('  ·  ')

  return (
    <DetailHero
      image={movie.backdrop_path || movie.poster_path}
      landscape={!!movie.backdrop_path}
      title={movie.title}
      subtitle={movie.original_title && movie.original_title !== movie.title ? movie.original_title : undefined}
      meta={meta}
      description={movie.description}
      actions={
        <>
          <button className="btn btn-primary" onClick={() => navigate(`/movies/${id}/watch`)} style={{ gap: '0.5rem' }}>
            <RiPlayFill /> Play
          </button>
          <WatchlistButton mediaType="movie" mediaId={id} />
        </>
      }
    />
  )
}
