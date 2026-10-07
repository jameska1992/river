// Backend-independent playback model. Nothing in this file may import a
// playback backend (Mediabunny, HTMLVideoElement specifics) — the React UI and
// any future River-level behaviour (progress reporting, watch parties,
// recovery) depend only on these types.

export type PlaybackStatus =
  | 'idle'
  | 'loading'
  | 'playing'
  | 'paused'
  | 'buffering'
  | 'ended'
  | 'error'

export type EngineKind = 'mediabunny' | 'native'

export type TrackType = 'video' | 'audio' | 'subtitle'

/** A River-friendly description of one track, decoupled from the backend's own track objects. */
export interface MediaTrackInfo {
  /** Opaque, engine-scoped identifier. Only meaningful to the engine that produced it. */
  id: string
  type: TrackType
  /** BCP-47 / ISO 639 code, or undefined when the container says "und". */
  language?: string
  label: string
  /** Short codec name, e.g. "avc" / "aac". */
  codec?: string
  /** Full codec string when known, e.g. "avc1.640028" / "mp4a.40.2". */
  codecString?: string
  isDefault?: boolean
  // Video
  width?: number
  height?: number
  // Audio
  channels?: number
  sampleRate?: number
}

export interface TimeRange { start: number; end: number }

export type PlaybackErrorCode =
  /** The backend can't play this media in this browser; a different engine might. */
  | 'unsupported'
  /** A network/HTTP failure fetching media bytes. */
  | 'network'
  /** Authentication failed and could not be refreshed. */
  | 'auth'
  /** The decoder or demuxer failed on the media data. */
  | 'decode'
  /** Anything else. */
  | 'unknown'

export interface PlaybackError {
  code: PlaybackErrorCode
  message: string
  /** True if reloading the pipeline at the current position is worth trying. */
  recoverable: boolean
  cause?: unknown
}

/** Feature flags an engine advertises so the UI can hide what it can't do instead of failing silently. */
export interface EngineCapabilities {
  audioTrackSwitching: boolean
  playbackRate: boolean
  pictureInPicture: boolean
}

/** Diagnostics for the debug panel. Free-form so engines can report what they know. */
export interface EngineDiagnostics {
  sourceKind?: string
  bytesFetched?: number
  requestCount?: number
  fileSize?: number
  decoder?: string
  droppedFrames?: number
  /**
   * Presented-frame timestamp vs the master clock at the moment it was drawn,
   * over frames drawn while playing. Audio is scheduled sample-exactly on the
   * same clock, so this is the effective A/V offset.
   */
  avSync?: { meanMs: number; maxMs: number; samples: number }
  /** Audio buffers that had to start part-way through because they were decoded late. */
  lateAudioStarts?: number
  /** Audio output latency compensated for when presenting video. */
  audioOutputLatencyMs?: number
  notes?: string[]
}

export interface PlaybackState {
  status: PlaybackStatus
  /** Current media time in seconds. */
  position: number
  /** Media duration in seconds; 0 until known. */
  duration: number
  /** Ranges of media time the engine can play without fetching/decoding more. */
  buffered: TimeRange[]
  /** Seconds of playable media ahead of `position`. */
  bufferedAhead: number
  volume: number
  muted: boolean
  playbackRate: number
  /** True while a seek is in flight. */
  seeking: boolean
  videoTrack: MediaTrackInfo | null
  audioTracks: MediaTrackInfo[]
  activeAudioTrackId: string | null
  error: PlaybackError | null
}

export const initialPlaybackState: PlaybackState = {
  status: 'idle',
  position: 0,
  duration: 0,
  buffered: [],
  bufferedAhead: 0,
  volume: 1,
  muted: false,
  playbackRate: 1,
  seeking: false,
  videoTrack: null,
  audioTracks: [],
  activeAudioTrackId: null,
  error: null,
}

/**
 * Where to play from. URLs are produced by a function rather than stored, so
 * every (re)connection picks up the current stream token — the token can be
 * rotated by an auth refresh while a long playback session is open.
 */
export interface PlaybackSource {
  /** River media id (movie/episode); used for diagnostics and keying. */
  mediaId: string
  title?: string
  /** Builds the authenticated stream URL. */
  resolveUrl: () => string
  /** Called on a 401 before retrying; should rotate the stream token. */
  refreshAuth?: () => Promise<void>
  /**
   * River's per-language variant files (video + one audio stream). Only the
   * native engine needs these — browsers without AudioTrackList can't switch
   * streams inside one MP4, so it reloads onto a variant instead.
   */
  audioVariants?: AudioVariant[]
}

export interface AudioVariant {
  id: string
  language: string
  label: string
  resolveUrl: () => string
}

export interface LoadOptions {
  /** Seek here once the media is ready. */
  startAt?: number
  autoplay?: boolean
}
