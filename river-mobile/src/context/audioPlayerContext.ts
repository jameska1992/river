import { createContext, useContext } from 'react'
import type { AudioItem } from '../util/audio'

export interface AudioPlayerState {
  current: AudioItem | null
  queue: AudioItem[]
  index: number
  playing: boolean
  position: number
  duration: number
  expanded: boolean
  // Start a new queue at startIndex (docks the mini-player; does not auto-expand).
  playQueue: (items: AudioItem[], startIndex: number) => void
  toggle: () => void
  // Absolute seek, in seconds.
  seek: (seconds: number) => void
  // Relative skip, in seconds (clamped to the track).
  skip: (deltaSeconds: number) => void
  next: () => void
  prev: () => void
  jumpTo: (index: number) => void
  expand: () => void
  collapse: () => void
  close: () => void
}

export const AudioPlayerContext = createContext<AudioPlayerState | null>(null)

export function useAudioPlayer(): AudioPlayerState {
  const ctx = useContext(AudioPlayerContext)
  if (!ctx) throw new Error('useAudioPlayer must be used within AudioPlayerProvider')
  return ctx
}
