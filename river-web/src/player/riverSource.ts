import type { AudioTrack, RiverClient } from '../api'
import type { PlaybackSource } from './core/types'

type StreamApi = Pick<RiverClient, 'movieStreamUrl' | 'audioTrackStreamUrl' | 'refreshStreamToken'>

/**
 * Builds a PlaybackSource for a River movie's transcode. Each resolveUrl()
 * call re-reads the current stream token, so a refresh mid-session is picked
 * up by the next Range request.
 */
export function movieSource(api: StreamApi, movieId: string, title?: string, audioTracks: AudioTrack[] = []): PlaybackSource {
  return {
    mediaId: movieId,
    title,
    resolveUrl: () => api.movieStreamUrl(movieId),
    refreshAuth: () => api.refreshStreamToken(),
    audioVariants: audioTracks.map(t => ({
      id: t.id,
      language: t.language,
      label: t.label || t.language,
      resolveUrl: () => api.audioTrackStreamUrl(t.id),
    })),
  }
}
