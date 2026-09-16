import { useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { VideoPlayer } from '../components/VideoPlayer'

export default function MoviePlayerPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { data: movie } = useAsync(() => api.getMovie(id), [id])

  const fetchSubtitles = useCallback(() => api.getMovieSubtitles(id), [id])
  const fetchAudioTracks = useCallback(() => api.getMovieAudioTracks(id), [id])
  const buildStreamUrl = useCallback(() => api.movieStreamUrl(id), [id])

  if (!id) return null

  return (
    <VideoPlayer
      streamUrl={api.movieStreamUrl(id)}
      buildStreamUrl={buildStreamUrl}
      title={movie?.title ?? ''}
      progressKind="movie"
      progressId={id}
      fetchSubtitles={fetchSubtitles}
      fetchAudioTracks={fetchAudioTracks}
      // Exit to the movie's detail page (replace so Back there goes upstream,
      // not back into the player — works even on a direct deep link).
      onExit={() => navigate(`/movies/${id}`, { replace: true })}
    />
  )
}
