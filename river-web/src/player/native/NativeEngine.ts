import { EngineError, toPlaybackError, type PlaybackEngine } from '../core/PlaybackEngine'
import { PlaybackStore } from '../core/PlaybackStore'
import { bufferedAhead, clampSeek, fromTimeRanges } from '../core/time'
import type {
  EngineCapabilities, EngineDiagnostics, LoadOptions, MediaTrackInfo, PlaybackSource, PlaybackState,
} from '../core/types'

interface NativeAudioTrack { id: string; label: string; language: string; enabled: boolean }
interface NativeAudioTrackList { length: number; [i: number]: NativeAudioTrack }
type VideoWithTracks = HTMLVideoElement & { audioTracks?: NativeAudioTrackList }

// MediaError codes (spec values; not every environment exposes the MediaError global).
const MEDIA_ERR_ABORTED = 1
const MEDIA_ERR_DECODE = 3
const MEDIA_ERR_SRC_NOT_SUPPORTED = 4

/**
 * Plays through a plain <video> element — the same mechanism as the
 * production watch pages — behind the PlaybackEngine interface. It's the
 * fallback when WebCodecs can't handle the media, and the reference the
 * Mediabunny engine is compared against.
 *
 * Audio tracks: where the browser exposes AudioTrackList (Safari) the engine
 * toggles tracks in place; elsewhere it reloads onto River's per-language
 * variant file at the current position, exactly like MovieWatchPage.
 */
export class NativeEngine implements PlaybackEngine {
  readonly kind = 'native' as const
  readonly capabilities: EngineCapabilities

  private readonly store = new PlaybackStore()
  private readonly video: VideoWithTracks
  private source: PlaybackSource | null = null
  /** Which URL is loaded: the main stream, or an audio variant id. */
  private activeVariant: string | null = null
  private readonly cleanup: (() => void)[] = []
  private pending: { startAt?: number; play?: boolean } | null = null
  private destroyed = false
  /** Set once the current source has produced metadata — i.e. the browser *can* play it. */
  private hadMetadata = false
  /**
   * Last healthy position + play intent — what reload() restores. Frozen from
   * a media error until a reload has loaded, because by then the element's
   * own currentTime/paused describe the broken (or freshly reset) element.
   */
  private resume = { position: 0, playing: false }
  private frozen = false

  constructor() {
    this.video = document.createElement('video') as VideoWithTracks
    this.video.playsInline = true
    this.video.preload = 'auto'
    this.video.style.cssText = 'width:100%;height:100%;display:block;object-fit:contain;background:#000'
    this.capabilities = {
      audioTrackSwitching: true,
      playbackRate: true,
      pictureInPicture: typeof document !== 'undefined' && !!document.pictureInPictureEnabled,
    }
    this.bindEvents()
  }

  /** Exposed for features that need the real element (PiP, casting). */
  get element(): HTMLVideoElement { return this.video }

  attach(container: HTMLElement): void {
    container.appendChild(this.video)
  }

  getState = (): PlaybackState => this.store.getState()
  subscribe = (l: () => void): (() => void) => this.store.subscribe(l)

  getDiagnostics(): EngineDiagnostics {
    const q = this.video.getVideoPlaybackQuality?.()
    return {
      sourceKind: 'HTMLVideoElement src (browser-managed HTTP Range)',
      decoder: 'Browser media pipeline',
      droppedFrames: q?.droppedVideoFrames,
    }
  }

  async load(source: PlaybackSource, options: LoadOptions = {}): Promise<void> {
    if (this.destroyed) throw new EngineError('unknown', 'Engine destroyed', { recoverable: false })
    this.source = source
    this.activeVariant = null
    this.hadMetadata = false
    this.resume = { position: options.startAt ?? 0, playing: !!options.autoplay }
    this.frozen = false
    this.store.reset({ status: 'loading' })
    const audioTracks: MediaTrackInfo[] = (source.audioVariants ?? []).map(v => ({
      id: v.id, type: 'audio', language: v.language, label: v.label || v.language,
    }))
    this.store.update({ audioTracks, activeAudioTrackId: audioTracks[0]?.id ?? null })
    await this.setSrc(source.resolveUrl(), { startAt: options.startAt, play: options.autoplay })
  }

  async reload(): Promise<void> {
    if (!this.source) return
    // Pre-emptively rotate the stream token (a <video> can't react to a 401).
    // Skipped while offline: RiverClient.doRefresh clears the whole session
    // on *any* failure, so a refresh attempted without connectivity logs the
    // user out.
    if (typeof navigator === 'undefined' || navigator.onLine) {
      try { await this.source.refreshAuth?.() } catch { /* still try — the token may be valid */ }
    }
    const url = this.currentUrl()
    if (!url) return
    const sep = url.includes('?') ? '&' : '?'
    this.frozen = true
    await this.setSrc(`${url}${sep}_r=${Date.now()}`, { startAt: this.resume.position, play: this.resume.playing })
  }

  async play(): Promise<void> {
    if (this.video.ended) this.video.currentTime = 0
    try {
      await this.video.play()
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      throw new EngineError('unknown', err instanceof Error ? err.message : String(err), { recoverable: false, cause: err })
    }
  }

  pause(): void { this.video.pause() }

  seek(position: number): Promise<void> {
    const target = clampSeek(position, this.video.duration || 0)
    this.resume.position = target
    return new Promise(resolve => {
      const done = () => { this.video.removeEventListener('seeked', done); resolve() }
      this.video.addEventListener('seeked', done)
      this.video.currentTime = target
      this.store.update({ position: target, seeking: true })
    })
  }

  setVolume(volume: number): void { this.video.volume = Math.min(Math.max(volume, 0), 1) }
  setMuted(muted: boolean): void { this.video.muted = muted }
  setPlaybackRate(rate: number): void { this.video.playbackRate = rate }

  async setAudioTrack(trackId: string): Promise<void> {
    const variants = this.source?.audioVariants ?? []
    const idx = variants.findIndex(v => v.id === trackId)
    if (idx < 0) throw new EngineError('unknown', `Unknown audio track ${trackId}`, { recoverable: false })
    const native = this.video.audioTracks
    if (native && native.length === variants.length) {
      for (let i = 0; i < native.length; i++) native[i].enabled = i === idx
      this.store.update({ activeAudioTrackId: trackId })
      return
    }
    this.activeVariant = trackId
    this.store.update({ activeAudioTrackId: trackId })
    await this.setSrc(variants[idx].resolveUrl(), { startAt: this.video.currentTime, play: !this.video.paused })
  }

  destroy(): void {
    if (this.destroyed) return
    this.destroyed = true
    for (const fn of this.cleanup) fn()
    this.store.clear()
    this.video.pause()
    this.video.removeAttribute('src')
    this.video.load()
    this.video.remove()
  }

  private currentUrl(): string | undefined {
    if (!this.source) return undefined
    if (this.activeVariant) return this.source.audioVariants?.find(v => v.id === this.activeVariant)?.resolveUrl()
    return this.source.resolveUrl()
  }

  private setSrc(url: string, pending: { startAt?: number; play?: boolean }): Promise<void> {
    this.pending = pending
    return new Promise((resolve, reject) => {
      const ok = () => { off(); resolve() }
      const bad = () => { off(); reject(new EngineError('network', this.mediaErrorMessage(), { recoverable: true })) }
      const off = () => {
        this.video.removeEventListener('loadedmetadata', ok)
        this.video.removeEventListener('error', bad)
      }
      this.video.addEventListener('loadedmetadata', ok)
      this.video.addEventListener('error', bad)
      this.video.src = url
    })
  }

  private mediaErrorMessage(): string {
    const e = this.video.error
    return e ? `Media error ${e.code}${e.message ? `: ${e.message}` : ''}` : 'Media error'
  }

  private on<K extends keyof HTMLMediaElementEventMap>(type: K, fn: () => void): void {
    this.video.addEventListener(type, fn)
    this.cleanup.push(() => this.video.removeEventListener(type, fn))
  }

  private bindEvents(): void {
    const v = this.video
    const syncBuffer = () => {
      const buffered = fromTimeRanges(v.buffered)
      this.store.update({ buffered, bufferedAhead: bufferedAhead(buffered, v.currentTime) })
    }
    this.on('loadedmetadata', () => {
      this.hadMetadata = true
      this.store.update({
        duration: v.duration,
        videoTrack: { id: 'video', type: 'video', label: 'Video', width: v.videoWidth, height: v.videoHeight },
      })
      const p = this.pending
      this.pending = null
      if (p?.startAt && p.startAt > 0) v.currentTime = p.startAt
      if (p?.play) void v.play().catch(() => {})
      if (!p?.play && v.paused) this.store.update({ status: 'paused' })
      this.frozen = false
    })
    this.on('play', () => { if (!this.frozen) this.resume.playing = true; this.store.update({ status: 'playing' }) })
    this.on('playing', () => { if (!this.frozen) this.resume.playing = true; this.store.update({ status: 'playing' }) })
    this.on('pause', () => {
      if (v.ended) return
      if (this.frozen) return // the broken/reset element's pause isn't the user's
      this.resume.playing = false
      this.store.update({ status: 'paused', position: v.currentTime })
    })
    this.on('waiting', () => { if (!v.paused) this.store.update({ status: 'buffering' }) })
    this.on('ended', () => this.store.update({ status: 'ended', position: v.duration }))
    this.on('seeking', () => this.store.update({ seeking: true }))
    this.on('seeked', () => { this.store.update({ seeking: false, position: v.currentTime }); syncBuffer() })
    this.on('timeupdate', () => {
      if (!v.seeking && !this.frozen) {
        this.resume.position = v.currentTime
        this.store.update({ position: v.currentTime })
      }
      syncBuffer()
    })
    this.on('progress', syncBuffer)
    this.on('durationchange', () => this.store.update({ duration: v.duration || 0 }))
    this.on('volumechange', () => this.store.update({ volume: v.volume, muted: v.muted }))
    this.on('ratechange', () => this.store.update({ playbackRate: v.playbackRate }))
    this.on('error', () => {
      const code = v.error?.code
      // ABORTED fires on our own src swaps; not a fault.
      if (code === MEDIA_ERR_ABORTED) return
      this.frozen = true
      // Firefox also reports a network failure mid-playback as
      // SRC_NOT_SUPPORTED, so that code only means "unsupported" before the
      // source has ever loaded.
      const kind = code === MEDIA_ERR_SRC_NOT_SUPPORTED && !this.hadMetadata ? 'unsupported'
        : code === MEDIA_ERR_DECODE ? 'decode' : 'network'
      this.store.update({ status: 'error', error: toPlaybackError(new EngineError(kind, this.mediaErrorMessage()), kind) })
    })
  }
}
