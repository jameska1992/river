import { initialPlaybackState, type PlaybackState } from './types'

export type Listener = () => void

/**
 * Minimal observable state container shared by every engine. Engines write
 * via `update`; the UI reads via `getState` + `subscribe` (shaped for React's
 * useSyncExternalStore). State objects are replaced, never mutated, so
 * identity comparison is a valid change check.
 */
export class PlaybackStore {
  private state: PlaybackState
  private readonly listeners = new Set<Listener>()

  constructor(initial: Partial<PlaybackState> = {}) {
    this.state = { ...initialPlaybackState, ...initial }
  }

  getState = (): PlaybackState => this.state

  subscribe = (listener: Listener): (() => void) => {
    this.listeners.add(listener)
    return () => { this.listeners.delete(listener) }
  }

  /** Shallow-merges `patch`; notifies only if some field actually changed. */
  update(patch: Partial<PlaybackState>): void {
    let changed = false
    for (const key of Object.keys(patch) as (keyof PlaybackState)[]) {
      if (!Object.is(this.state[key], patch[key])) { changed = true; break }
    }
    if (!changed) return
    this.state = { ...this.state, ...patch }
    for (const l of [...this.listeners]) l()
  }

  reset(patch: Partial<PlaybackState> = {}): void {
    const { volume, muted } = this.state
    // Volume/mute are user preferences, not media state — keep them across loads.
    this.state = { ...initialPlaybackState, volume, muted, ...patch }
    for (const l of [...this.listeners]) l()
  }

  clear(): void {
    this.listeners.clear()
  }
}
