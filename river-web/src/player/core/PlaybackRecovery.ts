import type { PlaybackEngine } from './PlaybackEngine'

const DEFAULT_COOLDOWN_MS = 8000
const DEFAULT_MAX_BACKOFF_MS = 60000
const STUCK_CHECK_MS = 1500

export interface RecoveryOptions {
  /** Minimum gap between automatic reloads, so a genuinely dead stream can't spin. Doubles per consecutive failure. */
  cooldownMs?: number
  /** Ceiling for the doubling cooldown. */
  maxBackoffMs?: number
  now?: () => number
  onRecover?: (reason: string) => void
}

/**
 * Engine-agnostic media recovery — the PlaybackEngine-level counterpart of
 * useMediaRecovery. It watches an engine's state and calls `engine.reload()`
 * (rebuild the pipeline at the current position, keep play/pause) when:
 *
 *  - the engine reports a recoverable error (network drop, decoder reset) —
 *    retried with exponential backoff while the error persists, and
 *    immediately when the browser comes back online;
 *  - the tab becomes visible again while "playing" but the position isn't
 *    moving (device sleep, background suspension).
 *
 * The engines themselves already absorb short blips (the Range reader retries
 * and refreshes expired stream tokens), so this only fires for real failures.
 */
export class PlaybackRecovery {
  private readonly engine: PlaybackEngine
  private readonly baseCooldownMs: number
  private readonly maxBackoffMs: number
  private readonly now: () => number
  private readonly onRecover?: (reason: string) => void
  private cooldownMs: number
  private lastAttempt = -Infinity
  private inFlight = false
  private readonly unsubs: (() => void)[] = []
  private stuckTimer: ReturnType<typeof setTimeout> | null = null
  private retryTimer: ReturnType<typeof setTimeout> | null = null

  constructor(engine: PlaybackEngine, opts: RecoveryOptions = {}) {
    this.engine = engine
    this.baseCooldownMs = opts.cooldownMs ?? DEFAULT_COOLDOWN_MS
    this.cooldownMs = this.baseCooldownMs
    this.maxBackoffMs = opts.maxBackoffMs ?? DEFAULT_MAX_BACKOFF_MS
    this.now = opts.now ?? Date.now
    this.onRecover = opts.onRecover

    this.unsubs.push(engine.subscribe(() => {
      const { status, error } = engine.getState()
      if (status === 'playing') this.cooldownMs = this.baseCooldownMs // healthy again
      if (status === 'error' && error?.recoverable) this.scheduleRetry(`error: ${error.message}`)
    }))

    const onVisible = () => {
      if (document.visibilityState !== 'visible') return
      const { status, position } = engine.getState()
      if (status !== 'playing') return
      if (this.stuckTimer) clearTimeout(this.stuckTimer)
      this.stuckTimer = setTimeout(() => {
        const s = engine.getState()
        if (s.status === 'playing' && s.position === position) void this.recover('stuck after resume')
      }, STUCK_CHECK_MS)
    }
    document.addEventListener('visibilitychange', onVisible)
    this.unsubs.push(() => document.removeEventListener('visibilitychange', onVisible))

    // Connectivity is back: don't wait out the backoff.
    const onOnline = () => {
      const { status, error } = engine.getState()
      if (status === 'error' && error?.recoverable) void this.recover('back online', true)
    }
    window.addEventListener('online', onOnline)
    this.unsubs.push(() => window.removeEventListener('online', onOnline))
  }

  /**
   * Reloads unless one is in flight or one happened within the cooldown
   * (`force` skips the cooldown). Returns whether it ran.
   */
  async recover(reason: string, force = false): Promise<boolean> {
    if (this.inFlight) return false
    const t = this.now()
    if (!force && t - this.lastAttempt < this.cooldownMs) return false
    this.clearRetry()
    this.lastAttempt = t
    this.inFlight = true
    this.onRecover?.(reason)
    try {
      await this.engine.reload()
      return true
    } catch {
      // Back off further, then try again if the engine is still in error.
      this.cooldownMs = Math.min(this.cooldownMs * 2, this.maxBackoffMs)
      return false
    } finally {
      this.inFlight = false
      const { status, error } = this.engine.getState()
      if (status === 'error' && error?.recoverable) this.scheduleRetry(`retry: ${error.message}`)
    }
  }

  /** Recovers now if allowed, otherwise once the cooldown has elapsed. */
  private scheduleRetry(reason: string): void {
    if (this.inFlight || this.retryTimer) return
    const wait = Math.max(0, this.lastAttempt + this.cooldownMs - this.now())
    if (wait === 0) { void this.recover(reason); return }
    this.retryTimer = setTimeout(() => {
      this.retryTimer = null
      const { status, error } = this.engine.getState()
      if (status === 'error' && error?.recoverable) void this.recover(reason)
    }, wait)
  }

  private clearRetry(): void {
    if (this.retryTimer) clearTimeout(this.retryTimer)
    this.retryTimer = null
  }

  dispose(): void {
    if (this.stuckTimer) clearTimeout(this.stuckTimer)
    this.clearRetry()
    for (const u of this.unsubs) u()
    this.unsubs.length = 0
  }
}
