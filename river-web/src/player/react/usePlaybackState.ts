import { useSyncExternalStore } from 'react'
import type { PlaybackEngine } from '../core/PlaybackEngine'
import { initialPlaybackState, type PlaybackState } from '../core/types'

const noopSubscribe = () => () => {}
const idle = () => initialPlaybackState

/** Subscribes a component to an engine's state. Re-renders only when the engine publishes a change. */
export function usePlaybackState(engine: PlaybackEngine | null): PlaybackState {
  return useSyncExternalStore(engine ? engine.subscribe : noopSubscribe, engine ? engine.getState : idle)
}
