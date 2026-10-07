import {
  AudioBufferSink, CanvasSink, Input, InputDisposedError, MP4, QTFF,
  type InputAudioTrack, type InputVideoTrack, type WrappedCanvas,
} from 'mediabunny'
import { EngineError, toPlaybackError, type PlaybackEngine } from '../core/PlaybackEngine'
import { PlaybackClock } from '../core/PlaybackClock'
import { PlaybackStore } from '../core/PlaybackStore'
import { clampSeek } from '../core/time'
import type {
  EngineCapabilities, EngineDiagnostics, LoadOptions, PlaybackSource, PlaybackState, PlaybackStatus,
} from '../core/types'
import { RiverRangeReader, createRiverSource } from './RiverInput'
import { describeAudioTrack, describeVideoTrack } from './MediaTrackAdapter'
import { detectWebCodecs } from './capabilities'

/** Seconds of decoded audio scheduled ahead of the playhead. Also the read-ahead backpressure. */
const AUDIO_LEAD = 2
/** How late the next video frame may be before playback is considered stalled. */
const STALL_GRACE = 0.25
/** Audio must have at least this much scheduled before a stall is cleared. */
const RESUME_AUDIO_LEAD = 0.5
/** If video decode falls this far behind the clock, re-seek the decoder instead of decoding every late frame. */
const VIDEO_RESYNC_LAG = 1
/** Max change in applied audio-latency compensation per rendered frame (see videoTime). */
const LATENCY_SLEW = 0.002
/** UI position updates while playing (the canvas itself updates every frame). */
const POSITION_EMIT_MS = 250

interface Session {
  source: PlaybackSource
  reader: RiverRangeReader
  input: Input
  videoTrack: InputVideoTrack
  canvasSink: CanvasSink
  audioTracks: Map<string, InputAudioTrack>
  audioSink: AudioBufferSink | null
  clock: PlaybackClock
  duration: number
}

/**
 * Plays River's H.264/AAC MP4 transcodes through Mediabunny + WebCodecs.
 *
 *   RiverRangeReader ─▶ Mediabunny Input (MP4 demux)
 *                         ├─ CanvasSink (VideoDecoder) ─▶ frames ─▶ <canvas>
 *                         └─ AudioBufferSink (AudioDecoder) ─▶ AudioBufferSourceNodes ─▶ GainNode ─▶ speakers
 *
 * Audio is the master clock: buffers are scheduled at exact AudioContext
 * times and the render loop presents whichever video frame is due at the
 * clock's current media time. Stalls (decode or network can't keep up)
 * suspend the AudioContext, which freezes the clock, so A/V stay aligned
 * through buffering instead of drifting.
 *
 * Async loops (video decode, audio scheduling) are tagged with a generation
 * number; seeking/pausing bumps the generation, so stale loops exit on their
 * next await instead of racing the new position.
 */
export class MediabunnyEngine implements PlaybackEngine {
  readonly kind = 'mediabunny' as const
  readonly capabilities: EngineCapabilities = {
    audioTrackSwitching: true,
    // Rate changes need pitch-preserving time-stretching, which WebAudio's
    // AudioBufferSourceNode doesn't do. Not in scope for the prototype.
    playbackRate: false,
    pictureInPicture: false,
  }

  private readonly store = new PlaybackStore()
  private canvas: HTMLCanvasElement | null = null
  private ctx2d: CanvasRenderingContext2D | null = null
  private audioCtx: AudioContext | null = null
  private gain: GainNode | null = null
  private session: Session | null = null
  /**
   * What to rebuild on reload(). Kept apart from the session because a
   * failed load (e.g. a recovery attempt while still offline) has no session,
   * and the next attempt must still know the media, position and play intent.
   */
  private resumePoint: { source: PlaybackSource; position: number; playing: boolean; audioId?: string } | null = null
  private destroyed = false

  // Intent and transient flags; `status` is derived from these in syncStatus().
  private wantPlaying = false
  private stalled = false
  private ended = false
  private loading = false

  private loadGen = 0
  private videoGen = 0
  private audioGen = 0
  private seekGen = 0

  // Video pipeline
  private videoIt: AsyncGenerator<WrappedCanvas, void, unknown> | null = null
  private nextFrame: WrappedCanvas | null = null
  private currentFrameEnd = 0
  private videoFetching = false
  private videoDone = false
  private droppedFrames = 0
  private sync = { sum: 0, max: 0, samples: 0 }
  private lateAudioStarts = 0

  // Audio pipeline
  private readonly queuedNodes = new Set<AudioBufferSourceNode>()
  private audioScheduledUntil = 0
  private audioDone = false

  private rafId: number | null = null
  /** Latency compensation currently applied to video; slews toward the reported value. */
  private appliedLatency = 0
  private lastEmit = 0
  private readonly notes: string[] = []

  attach(container: HTMLElement): void {
    if (!this.canvas) {
      this.canvas = document.createElement('canvas')
      this.canvas.style.cssText = 'width:100%;height:100%;display:block;object-fit:contain;background:#000'
      this.ctx2d = this.canvas.getContext('2d', { alpha: false })
    }
    container.appendChild(this.canvas)
  }

  getState = (): PlaybackState => this.store.getState()
  subscribe = (l: () => void): (() => void) => this.store.subscribe(l)

  getDiagnostics(): EngineDiagnostics {
    const stats = this.session?.reader.getStats()
    return {
      sourceKind: 'HTTP Range → Mediabunny CustomSource',
      bytesFetched: stats?.bytesFetched,
      requestCount: stats?.requestCount,
      fileSize: stats?.fileSize ?? undefined,
      decoder: 'WebCodecs (VideoDecoder + AudioDecoder)',
      droppedFrames: this.droppedFrames,
      avSync: this.sync.samples ? {
        meanMs: (this.sync.sum / this.sync.samples) * 1000,
        maxMs: this.sync.max * 1000,
        samples: this.sync.samples,
      } : undefined,
      lateAudioStarts: this.lateAudioStarts,
      audioOutputLatencyMs: this.audioCtx ? this.appliedLatency * 1000 : undefined,
      notes: [...this.notes],
    }
  }

  // ─── Loading ──────────────────────────────────────────────────────────────

  async load(source: PlaybackSource, options: LoadOptions = {}, preferredAudioId?: string): Promise<void> {
    this.assertAlive()
    const gen = ++this.loadGen
    this.teardownSession()
    this.notes.length = 0
    this.droppedFrames = 0
    this.sync = { sum: 0, max: 0, samples: 0 }
    this.lateAudioStarts = 0
    this.loading = true
    this.ended = false
    this.wantPlaying = false
    this.resumePoint = { source, position: options.startAt ?? 0, playing: !!options.autoplay, audioId: preferredAudioId }
    // Show the target position during the (re)load rather than flashing 0:00.
    this.store.reset({ status: 'loading', position: options.startAt ?? 0 })

    const support = detectWebCodecs()
    if (!support.videoDecoder || !support.audioContext) {
      this.loading = false
      const err = new EngineError('unsupported', `WebCodecs playback unavailable: ${support.missing.join(', ')}`, { recoverable: false })
      this.store.update({ status: 'error', error: err.toPlaybackError() })
      throw err
    }

    const reader = new RiverRangeReader({ resolveUrl: source.resolveUrl, refreshAuth: source.refreshAuth })
    const input = new Input({ source: createRiverSource(reader), formats: [MP4, QTFF] })

    try {
      const videoTrack = await input.getPrimaryVideoTrack()
      if (!videoTrack) throw new EngineError('unsupported', 'No video track in media', { recoverable: false })
      if (!(await videoTrack.canDecode())) {
        const codec = await videoTrack.getCodecParameterString()
        throw new EngineError('unsupported', `This browser's WebCodecs can't decode the video (${codec ?? 'unknown codec'})`, { recoverable: false })
      }

      // Every audio stream River muxed into the transcode is selectable here —
      // no per-language variant file needed (unlike the native player).
      const allAudio = await input.getAudioTracks()
      const audioTracks = new Map<string, InputAudioTrack>()
      const audioInfos = []
      for (const [i, t] of allAudio.entries()) {
        if (await t.canDecode()) {
          audioTracks.set(String(t.id), t)
          audioInfos.push(await describeAudioTrack(t, i))
        } else {
          this.notes.push(`Audio track ${t.id} (${(await t.getCodecParameterString()) ?? 'unknown'}) not decodable by this browser; hidden`)
        }
      }
      if (allAudio.length > 0 && audioTracks.size === 0) {
        throw new EngineError('unsupported', "This browser's WebCodecs can't decode any of the audio tracks", { recoverable: false })
      }

      let activeAudio: InputAudioTrack | null = null
      if (preferredAudioId && audioTracks.has(preferredAudioId)) {
        activeAudio = audioTracks.get(preferredAudioId)!
      } else if (audioTracks.size > 0) {
        const primary = await input.getPrimaryAudioTrack({ filter: t => audioTracks.has(String(t.id)) })
        activeAudio = primary ?? audioTracks.values().next().value!
      }

      const [duration, videoInfo] = await Promise.all([input.computeDuration(), describeVideoTrack(videoTrack)])
      if (gen !== this.loadGen) { input.dispose(); return }

      if (this.canvas) {
        this.canvas.width = videoInfo.width ?? 1920
        this.canvas.height = videoInfo.height ?? 1080
      }

      if (activeAudio) this.ensureAudioGraph()
      const audioCtx = this.audioCtx
      const clock = new PlaybackClock(activeAudio && audioCtx ? () => audioCtx.currentTime : () => performance.now() / 1000)
      clock.setDuration(duration)

      this.session = {
        source, reader, input, videoTrack, clock, duration, audioTracks,
        canvasSink: new CanvasSink(videoTrack, { poolSize: 3 }),
        audioSink: activeAudio ? new AudioBufferSink(activeAudio) : null,
      }
      this.audioDone = !activeAudio

      this.store.update({
        duration,
        videoTrack: videoInfo,
        audioTracks: audioInfos,
        activeAudioTrackId: activeAudio ? String(activeAudio.id) : null,
      })

      const startAt = clampSeek(options.startAt ?? 0, duration)
      clock.set(startAt)
      await this.restartVideo(startAt)
      if (gen !== this.loadGen) return
      this.loading = false
      this.store.update({ position: startAt })
      this.syncStatus()
      if (options.autoplay) await this.play()
    } catch (err) {
      if (gen === this.loadGen) {
        this.loading = false
        if (this.session?.input === input) this.session = null
        input.dispose()
        const pe = toPlaybackError(err, 'network')
        this.store.update({ status: 'error', error: pe })
      } else {
        input.dispose()
      }
      throw err instanceof EngineError ? err : new EngineError('network', toPlaybackError(err).message, { cause: err })
    }
  }

  async reload(): Promise<void> {
    const s = this.session
    if (s && this.store.getState().status !== 'error') this.captureResumePoint()
    const rp = this.resumePoint
    if (!rp) return
    await this.load(rp.source, { startAt: rp.position, autoplay: rp.playing }, rp.audioId)
  }

  private captureResumePoint(): void {
    const s = this.session
    if (!s) return
    this.resumePoint = {
      source: s.source,
      position: s.clock.getTime(),
      playing: this.wantPlaying,
      audioId: this.store.getState().activeAudioTrackId ?? undefined,
    }
  }

  // ─── Transport ────────────────────────────────────────────────────────────

  async play(): Promise<void> {
    const s = this.session
    if (!s || this.loading) return
    if (this.ended || s.clock.getTime() >= s.duration - 0.05) {
      await this.seek(0)
    }
    this.wantPlaying = true
    this.ended = false

    if (s.audioSink && this.audioCtx) {
      try { await this.audioCtx.resume() } catch { /* reported below */ }
      if (this.audioCtx.state !== 'running') {
        // Autoplay policy: an AudioContext can only start from a user gesture.
        this.wantPlaying = false
        this.syncStatus()
        throw new EngineError('unknown', 'Audio output is blocked until the user interacts with the page', { recoverable: false })
      }
    }
    if (this.session !== s || !this.wantPlaying) return

    s.clock.start()
    this.startAudio(s.clock.getTime())
    if (!this.nextFrame && !this.videoFetching && !this.videoDone) void this.advanceVideo(this.videoGen)
    this.syncStatus()
    this.startLoop()
  }

  pause(): void {
    const s = this.session
    this.wantPlaying = false
    if (!s) return
    s.clock.stop()
    this.stopAudio()
    this.clearStall()
    this.stopLoop()
    this.emitPosition(true)
    this.syncStatus()
  }

  async seek(position: number): Promise<void> {
    const s = this.session
    if (!s) return
    const target = clampSeek(position, s.duration)
    const gen = ++this.seekGen
    const resume = this.wantPlaying

    // Freeze everything at the target; audio restarts only once the video
    // frame there is decoded, so they come back in sync.
    s.clock.stop()
    s.clock.set(target)
    this.stopAudio()
    this.clearStall()
    this.ended = false
    this.store.update({ seeking: true, position: target })
    this.syncStatus()

    try {
      await this.restartVideo(target)
    } catch (err) {
      if (gen === this.seekGen) this.fail(err)
      return
    }
    if (gen !== this.seekGen || this.session !== s) return // superseded by a newer seek

    this.store.update({ seeking: false })
    if (resume && this.wantPlaying) {
      s.clock.start()
      this.startAudio(target)
      this.startLoop()
    }
    this.syncStatus()
  }

  setVolume(volume: number): void {
    const v = Math.min(Math.max(volume, 0), 1)
    this.store.update({ volume: v })
    this.applyGain()
  }

  setMuted(muted: boolean): void {
    this.store.update({ muted })
    this.applyGain()
  }

  setPlaybackRate(rate: number): void {
    if (rate !== 1) this.notes.push('Playback rate is not supported by the Mediabunny engine yet')
  }

  async setAudioTrack(trackId: string): Promise<void> {
    const s = this.session
    const track = s?.audioTracks.get(trackId)
    if (!s || !track) throw new EngineError('unknown', `Unknown audio track ${trackId}`, { recoverable: false })
    if (this.store.getState().activeAudioTrackId === trackId) return
    // Swap only the audio pipeline; video keeps running and the new track
    // picks up at the clock's current time.
    this.stopAudio()
    s.audioSink = new AudioBufferSink(track)
    this.store.update({ activeAudioTrackId: trackId })
    if (this.wantPlaying && s.clock.isRunning) this.startAudio(s.clock.getTime())
  }

  destroy(): void {
    if (this.destroyed) return
    this.destroyed = true
    this.loadGen++
    this.teardownSession()
    this.store.clear()
    void this.audioCtx?.close().catch(() => {})
    this.audioCtx = null
    this.gain = null
    this.canvas?.remove()
    this.canvas = null
    this.ctx2d = null
  }

  // ─── Video ────────────────────────────────────────────────────────────────

  /** Restarts the decoder at `at` and presents the frame there. */
  private async restartVideo(at: number): Promise<void> {
    const s = this.session
    if (!s) return
    const gen = ++this.videoGen
    void this.videoIt?.return().catch(() => {})
    this.nextFrame = null
    this.videoFetching = false
    this.videoDone = false

    const it = s.canvasSink.canvases(at)
    this.videoIt = it
    const first = await it.next()
    if (gen !== this.videoGen) return
    if (first.done) { this.videoDone = true; return }
    this.draw(first.value)
    await this.advanceVideo(gen)
  }

  /** Pulls frames until one is in the future; draws late ones on the way. */
  private async advanceVideo(gen: number): Promise<void> {
    const s = this.session
    const it = this.videoIt
    if (!s || !it || this.videoFetching) return
    this.videoFetching = true
    try {
      for (;;) {
        const r = await it.next()
        if (gen !== this.videoGen) return
        if (r.done) { this.videoDone = true; return }
        const frame = r.value
        const now = this.videoTime(s)
        if (frame.timestamp > now || !s.clock.isRunning) {
          this.nextFrame = frame
          return
        }
        // Late frame.
        if (now - frame.timestamp > VIDEO_RESYNC_LAG) {
          // E.g. returning from a background tab: re-seek the decoder to the
          // playhead (nearest key frame) rather than decode minutes of frames.
          this.droppedFrames++
          this.videoFetching = false
          void this.restartVideo(now).catch(err => this.fail(err))
          return
        }
        this.droppedFrames++
        this.draw(frame)
      }
    } catch (err) {
      if (gen === this.videoGen) this.fail(err)
    } finally {
      if (gen === this.videoGen) this.videoFetching = false
    }
  }

  /**
   * The media time being *heard* right now: the audio clock minus the output
   * device's latency (tens of ms wired, 150–250 ms on Bluetooth). Video is
   * presented against this so pictures line up with the sound that's
   * actually coming out of the speakers, not with what was just scheduled.
   */
  private videoTime(s: Session): number {
    const t = s.clock.getTime()
    if (!s.clock.isRunning || !s.audioSink || !this.audioCtx) return t
    return Math.max(0, t - this.appliedLatency)
  }

  /**
   * Moves the applied compensation toward the reported latency gradually.
   * Browsers revise outputLatency after the device opens (Chromium reports 0,
   * then ~70 ms a few hundred ms in); applying that as a step would freeze or
   * skip a few frames, slewing hides it.
   */
  private updateLatency(): void {
    const target = this.outputLatency()
    const d = target - this.appliedLatency
    this.appliedLatency += Math.max(-LATENCY_SLEW, Math.min(LATENCY_SLEW, d))
  }

  private outputLatency(): number {
    const ctx = this.audioCtx
    if (!ctx) return 0
    // outputLatency is the full figure where supported; baseLatency is the
    // context's own processing latency only (Safari lacks outputLatency).
    return (ctx.outputLatency || 0) || ctx.baseLatency || 0
  }

  private draw(frame: WrappedCanvas): void {
    const c = this.canvas
    if (c && this.ctx2d) this.ctx2d.drawImage(frame.canvas, 0, 0, c.width, c.height)
    this.currentFrameEnd = frame.timestamp + frame.duration
    const clock = this.session?.clock
    if (clock?.isRunning && !this.stalled) {
      const offset = Math.abs(this.videoTime(this.session!) - frame.timestamp)
      this.sync.sum += offset
      this.sync.max = Math.max(this.sync.max, offset)
      this.sync.samples++
    }
  }

  // ─── Audio ────────────────────────────────────────────────────────────────

  private ensureAudioGraph(): void {
    if (this.audioCtx) return
    const ctx = new AudioContext({ latencyHint: 'playback' })
    const gain = ctx.createGain()
    gain.connect(ctx.destination)
    this.audioCtx = ctx
    this.gain = gain
    this.applyGain()
    // The OS/browser can suspend or interrupt the context (device sleep,
    // iOS interruptions). If that happens behind our back, reflect it as a
    // pause instead of a frozen "playing" state.
    ctx.addEventListener('statechange', () => {
      if (ctx.state !== 'running' && this.wantPlaying && !this.stalled && this.session?.audioSink) {
        this.pause()
      }
    })
  }

  private applyGain(): void {
    if (!this.gain || !this.audioCtx) return
    const { volume, muted } = this.store.getState()
    this.gain.gain.value = muted ? 0 : volume
  }

  private startAudio(from: number): void {
    const s = this.session
    if (!s?.audioSink || !this.audioCtx || !this.gain) return
    const gen = ++this.audioGen
    this.audioDone = false
    this.audioScheduledUntil = from
    void this.runAudio(gen, s, s.audioSink, from)
  }

  private async runAudio(gen: number, s: Session, sink: AudioBufferSink, from: number): Promise<void> {
    const ctx = this.audioCtx!
    const gain = this.gain!
    const it = sink.buffers(from)
    try {
      for await (const { buffer, timestamp, duration } of it) {
        if (gen !== this.audioGen) break
        const node = ctx.createBufferSource()
        node.buffer = buffer
        node.connect(gain)
        const when = s.clock.toSourceTime(timestamp)
        const now = ctx.currentTime
        if (when >= now) {
          node.start(when)
        } else if (now - when < buffer.duration) {
          // Partially late (e.g. first buffer after a seek): start mid-buffer.
          node.start(now, now - when)
          this.lateAudioStarts++
        } else {
          node.disconnect()
          this.audioScheduledUntil = timestamp + duration
          continue
        }
        this.queuedNodes.add(node)
        node.onended = () => { this.queuedNodes.delete(node) }
        this.audioScheduledUntil = timestamp + duration

        // Backpressure: don't decode further than AUDIO_LEAD ahead.
        while (gen === this.audioGen && timestamp - s.clock.getTime() >= AUDIO_LEAD) {
          await sleep(100)
        }
        if (gen !== this.audioGen) break
      }
      if (gen === this.audioGen) this.audioDone = true
    } catch (err) {
      if (gen === this.audioGen) this.fail(err)
    } finally {
      void it.return().catch(() => {})
    }
  }

  private stopAudio(): void {
    this.audioGen++
    for (const node of this.queuedNodes) {
      try { node.stop() } catch { /* not started */ }
      node.disconnect()
    }
    this.queuedNodes.clear()
  }

  // ─── Render loop, stalls, end ─────────────────────────────────────────────

  private startLoop(): void {
    if (this.rafId !== null) return
    const tick = () => {
      this.rafId = requestAnimationFrame(tick)
      this.onTick()
    }
    this.rafId = requestAnimationFrame(tick)
  }

  private stopLoop(): void {
    if (this.rafId !== null) cancelAnimationFrame(this.rafId)
    this.rafId = null
  }

  private onTick(): void {
    const s = this.session
    if (!s || !this.wantPlaying || this.store.getState().seeking) return
    const t = s.clock.getTime()
    if (s.audioSink) this.updateLatency()
    const tv = this.videoTime(s)

    if (this.nextFrame && this.nextFrame.timestamp <= tv) {
      this.draw(this.nextFrame)
      this.nextFrame = null
      void this.advanceVideo(this.videoGen)
    }

    if (this.videoDone && (this.audioDone || !s.audioSink) && t >= s.duration - 0.05) {
      this.onEnded()
      return
    }
    if (t >= s.duration) { this.onEnded(); return }

    const videoStarved = !this.videoDone && !this.nextFrame && tv > this.currentFrameEnd + STALL_GRACE
    const audioLead = this.audioScheduledUntil - t
    const audioStarved = !!s.audioSink && !this.audioDone && audioLead < 0.05
    if (!this.stalled && (videoStarved || audioStarved)) {
      this.enterStall()
    } else if (this.stalled) {
      const videoReady = this.videoDone || !!this.nextFrame
      const audioReady = !s.audioSink || this.audioDone || audioLead >= RESUME_AUDIO_LEAD
      if (videoReady && audioReady) this.clearStall()
    }

    this.emitPosition(false)
  }

  private enterStall(): void {
    const s = this.session
    if (!s) return
    this.stalled = true
    // Freezing the clock keeps A/V aligned while data catches up. With audio,
    // suspending the context freezes ctx.currentTime (and every scheduled node).
    if (s.audioSink && this.audioCtx) void this.audioCtx.suspend()
    else s.clock.stop()
    this.syncStatus()
  }

  private clearStall(): void {
    if (!this.stalled) return
    this.stalled = false
    const s = this.session
    if (s && this.wantPlaying) {
      if (s.audioSink && this.audioCtx) void this.audioCtx.resume()
      else s.clock.start()
    }
    this.syncStatus()
  }

  private onEnded(): void {
    const s = this.session
    if (!s) return
    s.clock.stop()
    s.clock.set(s.duration)
    this.wantPlaying = false
    this.ended = true
    this.stopAudio()
    this.stopLoop()
    this.emitPosition(true)
    this.syncStatus()
  }

  // ─── State helpers ────────────────────────────────────────────────────────

  private emitPosition(force: boolean): void {
    const s = this.session
    if (!s) return
    const now = performance.now()
    if (!force && now - this.lastEmit < POSITION_EMIT_MS) return
    this.lastEmit = now
    // Position (progress bar, subtitles, progress reporting) follows the
    // clock; only frame presentation is latency-compensated.
    const position = s.clock.getTime()
    const videoAhead = this.nextFrame ? Math.max(0, this.nextFrame.timestamp - position) : 0
    const ahead = s.audioSink ? Math.max(0, this.audioScheduledUntil - position) : videoAhead
    this.store.update({
      position,
      bufferedAhead: ahead,
      buffered: [{ start: position, end: Math.min(s.duration, position + ahead) }],
    })
  }

  private syncStatus(): void {
    const st = this.store.getState()
    let status: PlaybackStatus
    if (st.error && st.status === 'error') status = 'error'
    else if (this.loading) status = 'loading'
    else if (this.ended) status = 'ended'
    else if (this.wantPlaying && (this.stalled || st.seeking)) status = 'buffering'
    else if (this.wantPlaying) status = 'playing'
    else status = 'paused'
    this.store.update({ status })
  }

  private fail(err: unknown): void {
    if (err instanceof InputDisposedError || (err instanceof DOMException && err.name === 'AbortError')) return
    if (!this.session) return
    const pe = toPlaybackError(err, 'decode')
    // Remember where we were and whether we were playing, so a recovery
    // reload resumes exactly here.
    this.captureResumePoint()
    this.wantPlaying = false
    this.session.clock.stop()
    this.stopAudio()
    this.stopLoop()
    this.stalled = false
    this.store.update({ status: 'error', error: pe, seeking: false })
  }

  private teardownSession(): void {
    this.stopLoop()
    this.stopAudio()
    this.videoGen++
    this.seekGen++
    void this.videoIt?.return().catch(() => {})
    this.videoIt = null
    this.nextFrame = null
    this.videoFetching = false
    this.stalled = false
    this.session?.input.dispose()
    this.session = null
  }

  private assertAlive(): void {
    if (this.destroyed) throw new EngineError('unknown', 'Engine destroyed', { recoverable: false })
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise(r => setTimeout(r, ms))
}
