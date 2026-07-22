export {}

declare global {
  namespace cast {
    namespace framework {
      class CastContext {
        static getInstance(): CastContext
        setOptions(opts: { receiverApplicationId: string; autoJoinPolicy: string }): void
        getCurrentSession(): CastSession | null
        getSessionState(): string
        addEventListener(type: string, handler: () => void): void
      }
      class CastSession {
        loadMedia(request: chrome.cast.media.LoadRequest): Promise<unknown>
      }
      const CastContextEventType: { SESSION_STATE_CHANGED: string }
      const SessionState: { SESSION_STARTED: string; SESSION_RESUMED: string; SESSION_ENDED: string }
    }
  }

  namespace chrome {
    namespace cast {
      const AutoJoinPolicy: { ORIGIN_SCOPED: string }
      namespace media {
        const DEFAULT_MEDIA_RECEIVER_APP_ID: string
        class MediaInfo {
          constructor(contentId: string, contentType: string)
          metadata: unknown
        }
        class LoadRequest {
          constructor(mediaInfo: MediaInfo)
          currentTime: number
        }
        class MovieMediaMetadata {
          title: string
          images: Array<{ url: string }>
        }
        class TvShowMediaMetadata {
          seriesTitle: string
          season: number
          episode: number
          title: string
          images: Array<{ url: string }>
        }
        class MusicTrackMediaMetadata {
          title: string
          artist: string
          albumName: string
          images: Array<{ url: string }>
        }
        class GenericMediaMetadata {
          title: string
          images: Array<{ url: string }>
        }
      }
    }
  }

  interface Window {
    __onGCastApiAvailable?: (isAvailable: boolean) => void
  }
}

declare module 'react/jsx-runtime' {
  namespace JSX {
    interface IntrinsicElements {
      'google-cast-launcher': { className?: string; style?: Record<string, string> }
    }
  }
}
