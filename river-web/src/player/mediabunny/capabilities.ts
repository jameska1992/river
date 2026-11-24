import { canDecodeAudio, canDecodeVideo } from 'mediabunny'

export interface WebCodecsSupport {
  videoDecoder: boolean
  audioDecoder: boolean
  audioContext: boolean
  /** Human-readable names of what's missing. */
  missing: string[]
}

/** Synchronous feature detection — does the browser expose the APIs at all? */
export function detectWebCodecs(g: Record<string, unknown> = globalThis as unknown as Record<string, unknown>): WebCodecsSupport {
  const videoDecoder = typeof g.VideoDecoder === 'function'
  const audioDecoder = typeof g.AudioDecoder === 'function'
  const audioContext = typeof g.AudioContext === 'function'
  const missing: string[] = []
  if (!videoDecoder) missing.push('VideoDecoder')
  if (!audioDecoder) missing.push('AudioDecoder')
  if (!audioContext) missing.push('AudioContext')
  return { videoDecoder, audioDecoder, audioContext, missing }
}

export interface RiverCodecSupport {
  /** H.264 High@4.0 (River's 1080p transcode profile). */
  h264: boolean
  /** AAC-LC stereo 48 kHz. */
  aac: boolean
}

/**
 * Asks WebCodecs whether River's transcode output is decodable. This is a
 * cheap pre-flight for engine selection; the engine still checks the actual
 * tracks' decoder configs at load time.
 */
export async function probeRiverCodecs(): Promise<RiverCodecSupport> {
  const support = detectWebCodecs()
  if (!support.videoDecoder) return { h264: false, aac: false }
  const [h264, aac] = await Promise.all([
    canDecodeVideo('avc', { codec: 'avc1.640028', codedWidth: 1920, codedHeight: 1080 }).catch(() => false),
    support.audioDecoder
      ? canDecodeAudio('aac', { codec: 'mp4a.40.2', numberOfChannels: 2, sampleRate: 48000 }).catch(() => false)
      : Promise.resolve(false),
  ])
  return { h264, aac }
}
