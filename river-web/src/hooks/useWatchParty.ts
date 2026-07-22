import { useEffect, useRef, useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import type { WatchPartyMember } from '../api'

interface WPServerMsg {
  type: string
  position?: number
  playing?: boolean
  from?: string
  members?: WatchPartyMember[]
}

export function useWatchParty(
  roomId: string | undefined,
  videoRef: React.RefObject<HTMLVideoElement | null>,
  isHost: boolean,
  backPath: string,
) {
  const navigate = useNavigate()
  const [members, setMembers] = useState<WatchPartyMember[]>([])
  const [connected, setConnected] = useState(false)
  const socketRef = useRef<ReturnType<typeof api.openWatchPartySocket> | null>(null)
  // Refs so message handler always sees current values without re-registering
  const isHostRef = useRef(isHost)
  isHostRef.current = isHost
  const videoRefRef = useRef<React.RefObject<HTMLVideoElement | null>>(videoRef)
  videoRefRef.current = videoRef

  const sendCommand = useCallback((type: 'play' | 'pause' | 'seek', position: number) => {
    socketRef.current?.send(type, position)
  }, [])

  useEffect(() => {
    if (!roomId) return

    const socket = api.openWatchPartySocket(roomId)
    socketRef.current = socket

    // Apply a seek+play/pause once the video has enough metadata to accept
    // currentTime assignments. If the video isn't ready yet, defer via
    // loadedmetadata so the seek isn't silently dropped.
    const applySync = (v: HTMLVideoElement, position: number, playing: boolean) => {
      const doApply = () => {
        if (Math.abs(v.currentTime - position) > 2) v.currentTime = position
        if (playing && v.paused) v.play().catch(() => {})
        else if (!playing && !v.paused) v.pause()
      }
      if (v.readyState >= 1) {
        doApply()
      } else {
        const onReady = () => {
          v.removeEventListener('loadedmetadata', onReady)
          doApply()
        }
        v.addEventListener('loadedmetadata', onReady)
      }
    }

    socket.onMessage(raw => {
      const msg = raw as WPServerMsg
      const v = videoRefRef.current.current

      switch (msg.type) {
        case 'state':
          setConnected(true)
          if (msg.members) setMembers(msg.members)
          // Non-host syncs to the room's current position and play state.
          if (!isHostRef.current && v) {
            applySync(v, msg.position ?? 0, msg.playing ?? false)
          }
          break
        case 'members':
          if (msg.members) setMembers(msg.members)
          break
        case 'play':
          if (!isHostRef.current && v) {
            if (msg.position !== undefined) v.currentTime = msg.position
            v.play().catch(() => {})
          }
          break
        case 'pause':
          if (!isHostRef.current && v) {
            if (msg.position !== undefined) v.currentTime = msg.position
            v.pause()
          }
          break
        case 'seek':
          if (!isHostRef.current && v && msg.position !== undefined) {
            v.currentTime = msg.position
          }
          break
        case 'closed':
          socket.close()
          navigate(backPath, { replace: true })
          break
      }
    })

    return () => {
      socket.close()
      socketRef.current = null
    }
  }, [roomId]) // eslint-disable-line react-hooks/exhaustive-deps

  return { members, connected, sendCommand }
}
