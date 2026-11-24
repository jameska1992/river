import { EngineError, type PlaybackEngine } from './PlaybackEngine'
import type { EngineKind, LoadOptions, PlaybackSource } from './types'

export type EnginePreference = EngineKind | 'auto'

export type EngineFactory = (kind: EngineKind) => PlaybackEngine

export interface EngineSelection {
  engine: PlaybackEngine
  /** Why we ended up on this engine — for the debug panel. */
  reason: string
}

/**
 * The order engines are tried in. `auto` prefers Mediabunny only when the
 * browser exposes WebCodecs at all; per-media decodability is checked by the
 * Mediabunny engine itself at load time (and triggers the fallback below).
 */
export function engineOrder(pref: EnginePreference, webCodecsAvailable: boolean): EngineKind[] {
  if (pref === 'native') return ['native']
  if (pref === 'mediabunny') return ['mediabunny']
  return webCodecsAvailable ? ['mediabunny', 'native'] : ['native']
}

/**
 * Creates, attaches and loads the first engine in `order` that can play
 * `source`. An engine that rejects with code `'unsupported'` is destroyed and
 * the next one is tried; any other failure is surfaced as-is (falling back on
 * a network error would just hide the real problem).
 */
export async function loadWithFallback(
  order: EngineKind[],
  factory: EngineFactory,
  container: HTMLElement,
  source: PlaybackSource,
  options: LoadOptions,
  onEngine?: (engine: PlaybackEngine) => void,
): Promise<EngineSelection> {
  const skipped: string[] = []
  for (const [i, kind] of order.entries()) {
    const engine = factory(kind)
    engine.attach(container)
    onEngine?.(engine)
    try {
      await engine.load(source, options)
      const reason = skipped.length ? `fell back: ${skipped.join('; ')}` : i === 0 && order.length === 1 ? 'forced' : 'preferred'
      return { engine, reason }
    } catch (err) {
      const last = i === order.length - 1
      if (err instanceof EngineError && err.code === 'unsupported' && !last) {
        skipped.push(`${kind}: ${err.message}`)
        engine.destroy()
        continue
      }
      // Leave the failed engine in place so the UI can show its error state.
      return { engine, reason: `failed: ${err instanceof Error ? err.message : String(err)}` }
    }
  }
  throw new Error('No playback engine available')
}
