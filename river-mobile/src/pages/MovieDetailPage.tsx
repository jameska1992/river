import { useParams, useNavigate } from 'react-router-dom'
import { RiPlayFill } from 'react-icons/ri'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { DetailHero } from '../components/Detail'
import { PeopleRow } from '../components/People'
import { WatchlistButton } from '../components/WatchlistButton'
import { Loading, ErrorState } from '../components/States'
import { castToEntries, crewToEntries } from '../util/credits'

export default function MovieDetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { data, loading, error, reload } = useAsync(async () => {
    // Credits are best-effort — a movie without enriched metadata still shows.
    const [movie, credits] = await Promise.all([api.getMovie(id), api.getMovieCredits(id).catch(() => null)])
    return { movie, credits }
  }, [id])

  if (loading) return <Loading />
  if (error || !data) return <ErrorState message={error ?? 'Not found'} onRetry={reload} />

  const { movie, credits } = data
  const meta = [
    movie.year > 0 ? String(movie.year) : null,
    movie.runtime > 0 ? `${movie.runtime} min` : null,
    movie.rating > 0 ? `★ ${movie.rating.toFixed(1)}` : null,
    ...(movie.genres ?? []),
  ].filter(Boolean).join('  ·  ')

  return (
    <div>
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

      {credits && <PeopleRow title="Cast" people={castToEntries(credits.cast)} />}
      {credits && <PeopleRow title="Crew" people={crewToEntries(credits.crew)} />}
    </div>
  )
}
