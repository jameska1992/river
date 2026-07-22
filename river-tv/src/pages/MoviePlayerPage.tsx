import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type Movie } from '../api'
import { PlayerScreen } from '../components/PlayerScreen'

export default function MoviePlayerPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [movie, setMovie] = useState<Movie | null>(null)

  useEffect(() => {
    if (!id) return
    let alive = true
    api.getMovie(id).then(m => { if (alive) setMovie(m) }).catch(() => {})
    return () => { alive = false }
  }, [id])

  const fetchSubtitles = useCallback(() => api.getMovieSubtitles(id), [id])
  const fetchAudioTracks = useCallback(() => api.getMovieAudioTracks(id), [id])

  if (!id) return null

  return (
    <PlayerScreen
      streamUrl={api.movieStreamUrl(id)}
      title={movie?.title ?? ''}
      progressKind="movie"
      progressId={id}
      fetchSubtitles={fetchSubtitles}
      fetchAudioTracks={fetchAudioTracks}
      // Always exit to the movie's detail page (replace so Back from
      // there goes to the upstream browse/home, not back into the
      // player). Survives the case where the user opened the player
      // directly via a deep link with no history to pop.
      onExit={() => navigate(`/movies/${id}`, { replace: true })}
    />
  )
}
