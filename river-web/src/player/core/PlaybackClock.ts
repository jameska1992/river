/**
 * Maps a monotonically increasing time source (seconds) onto media time.
 *
 * The Mediabunny engine uses AudioContext.currentTime as the time source when
 * the media has audio — audio is the master clock, video frames are presented
 * against it, which is what keeps A/V in sync. Without audio it falls back to
 * performance.now().
 *
 * The clock is anchored at (sourceTime, mediaTime) when it starts running;
 * media time is then mediaAnchor + (now - sourceAnchor), clamped to the
 * duration when one is set.
 */
export class PlaybackClock {
  private readonly now: () => number
  private running = false
  private sourceAnchor = 0
  private mediaAnchor = 0
  private duration = Infinity

  constructor(now: () => number) {
    this.now = now
  }

  get isRunning(): boolean { return this.running }

  setDuration(d: number): void {
    this.duration = d > 0 ? d : Infinity
  }

  /** Current media time in seconds. */
  getTime(): number {
    const t = this.running ? this.mediaAnchor + (this.now() - this.sourceAnchor) : this.mediaAnchor
    return Math.min(Math.max(0, t), this.duration)
  }

  /** Starts (or re-anchors) the clock at the current media time. */
  start(): void {
    if (this.running) return
    this.sourceAnchor = this.now()
    this.running = true
  }

  /** Freezes media time at its current value. */
  stop(): void {
    if (!this.running) return
    this.mediaAnchor = this.getTime()
    this.running = false
  }

  /** Jumps to `t`; keeps running if it was running. */
  set(t: number): void {
    this.mediaAnchor = Math.min(Math.max(0, t), this.duration)
    this.sourceAnchor = this.now()
  }

  /**
   * Translates a media timestamp into the time source's timeline — used to
   * schedule AudioBufferSourceNodes at the exact context time a sample should
   * sound. Only meaningful while running.
   */
  toSourceTime(mediaTime: number): number {
    return this.sourceAnchor + (mediaTime - this.mediaAnchor)
  }
}
