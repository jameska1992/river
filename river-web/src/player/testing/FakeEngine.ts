import { vi } from 'vitest'
import type { PlaybackEngine } from '../core/PlaybackEngine'
import { PlaybackStore } from '../core/PlaybackStore'
import type { EngineKind, PlaybackState } from '../core/types'

// A scriptable engine used for the selection, recovery and UI tests.
export class FakeEngine implements PlaybackEngine {
  static instances: FakeEngine[] = []
  static loadBehaviour: Partial<Record<EngineKind, () => Promise<void>>> = {}

  readonly kind: EngineKind
  readonly capabilities = { audioTrackSwitching: true, playbackRate: true, pictureInPicture: false }
  readonly store = new PlaybackStore()
  attached: HTMLElement | null = null
  destroyed = false
  play = vi.fn(async () => { this.store.update({ status: 'playing' }) })
  pause = vi.fn(() => { this.store.update({ status: 'paused' }) })
  seek = vi.fn(async (p: number) => { this.store.update({ position: p }) })
  setVolume = vi.fn((v: number) => { this.store.update({ volume: v }) })
  setMuted = vi.fn((m: boolean) => { this.store.update({ muted: m }) })
  setPlaybackRate = vi.fn()
  setAudioTrack = vi.fn(async (id: string) => { this.store.update({ activeAudioTrackId: id }) })
  reload = vi.fn(async () => {})

  constructor(kind: EngineKind) {
    this.kind = kind
    FakeEngine.instances.push(this)
  }

  attach(c: HTMLElement) { this.attached = c }
  async load() {
    const b = FakeEngine.loadBehaviour[this.kind]
    if (b) await b()
    this.store.update({ status: 'paused', duration: 100 })
  }
  getState = () => this.store.getState()
  subscribe = (l: () => void) => this.store.subscribe(l)
  getDiagnostics() { return { sourceKind: 'fake' } }
  destroy() { this.destroyed = true }
  set(patch: Partial<PlaybackState>) { this.store.update(patch) }
}
