import type {
  EngineCapabilities, EngineDiagnostics, EngineKind, LoadOptions, PlaybackError,
  PlaybackSource, PlaybackState,
} from './types'

/**
 * The boundary between River's player UI and a playback backend.
 *
 * The UI never touches an HTMLVideoElement, WebCodecs or Mediabunny directly;
 * it drives an engine through these methods and renders from `getState()`,
 * re-rendering when `subscribe` fires.
 *
 * Rendering: the engine owns its output surface (a <canvas> or <video>) and
 * mounts it into the element passed to `attach`. Anything layered on top —
 * subtitles, controls, watch-party overlays — is the UI's job, so it works
 * the same for every engine.
 *
 * Subtitles are deliberately not part of this interface: River serves them as
 * sidecar WebVTT files, rendered by an overlay driven by `state.position`.
 *
 * Fullscreen likewise belongs to the UI container, not the engine (fullscreening
 * just the canvas would hide the controls).
 */
export interface PlaybackEngine {
  readonly kind: EngineKind
  readonly capabilities: EngineCapabilities

  /** Mounts the engine's render surface into `container`. Call before `load`. */
  attach(container: HTMLElement): void

  /**
   * Opens `source`. Rejects with an `EngineError` whose `code` is
   * `'unsupported'` if this engine can't play the media in this browser —
   * callers use that to fall back to another engine.
   */
  load(source: PlaybackSource, options?: LoadOptions): Promise<void>

  play(): Promise<void>
  pause(): void
  /** Resolves once the frame at `position` is presented. Positions are clamped to [0, duration]. */
  seek(position: number): Promise<void>

  setVolume(volume: number): void
  setMuted(muted: boolean): void
  setPlaybackRate(rate: number): void
  setAudioTrack(trackId: string): Promise<void>

  /**
   * Tears the media pipeline down and rebuilds it at the current position,
   * preserving play/pause — the primitive higher-level recovery is built on.
   */
  reload(): Promise<void>

  getState(): PlaybackState
  subscribe(listener: () => void): () => void
  getDiagnostics(): EngineDiagnostics

  /** Releases decoders, network requests, audio graph and DOM. Idempotent. */
  destroy(): void
}

export class EngineError extends Error implements PlaybackError {
  readonly code: PlaybackError['code']
  readonly recoverable: boolean
  readonly cause?: unknown

  constructor(code: PlaybackError['code'], message: string, opts: { recoverable?: boolean; cause?: unknown } = {}) {
    super(message)
    this.name = 'EngineError'
    this.code = code
    this.recoverable = opts.recoverable ?? (code === 'network' || code === 'decode')
    this.cause = opts.cause
  }

  toPlaybackError(): PlaybackError {
    return { code: this.code, message: this.message, recoverable: this.recoverable, cause: this.cause }
  }
}

/** Normalises anything thrown into a PlaybackError. */
export function toPlaybackError(err: unknown, fallback: PlaybackError['code'] = 'unknown'): PlaybackError {
  if (err instanceof EngineError) return err.toPlaybackError()
  const message = err instanceof Error ? err.message : String(err)
  return { code: fallback, message, recoverable: fallback === 'network' || fallback === 'decode', cause: err }
}
