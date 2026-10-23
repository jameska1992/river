import type { InputAudioTrack, InputVideoTrack } from 'mediabunny'
import type { MediaTrackInfo } from '../core/types'

// Keeps Mediabunny's track objects out of the UI: the engine maps them to
// plain MediaTrackInfo records once at load and keeps its own id → track map.

/** The fields the mapping needs, already resolved from Mediabunny's async getters. */
export interface RawTrackFacts {
  id: number
  type: 'video' | 'audio'
  languageCode: string
  name: string | null
  codec: string | null
  codecString: string | null
  isDefault: boolean
  width?: number
  height?: number
  channels?: number
  sampleRate?: number
}

let displayNames: Intl.DisplayNames | null | undefined

/** Human label for an ISO 639 code ("eng" / "en" → "English"); falls back to the code. */
export function languageLabel(code: string | undefined): string | undefined {
  if (!code) return undefined
  if (displayNames === undefined) {
    try { displayNames = new Intl.DisplayNames(['en'], { type: 'language' }) } catch { displayNames = null }
  }
  try {
    const name = displayNames?.of(code)
    if (name && name.toLowerCase() !== code.toLowerCase()) return name
  } catch { /* invalid code */ }
  return code
}

export function channelLabel(channels: number | undefined): string | undefined {
  switch (channels) {
    case undefined: return undefined
    case 1: return 'Mono'
    case 2: return 'Stereo'
    case 6: return '5.1'
    case 8: return '7.1'
    default: return `${channels}ch`
  }
}

export function toTrackInfo(f: RawTrackFacts, index: number): MediaTrackInfo {
  const language = f.languageCode && f.languageCode !== 'und' ? f.languageCode : undefined
  let label = f.name?.trim() || languageLabel(language) || `Track ${index + 1}`
  if (f.type === 'audio') {
    const ch = channelLabel(f.channels)
    if (ch && !label.includes(ch)) label = `${label} (${ch})`
  }
  return {
    id: String(f.id),
    type: f.type,
    language,
    label,
    codec: f.codec ?? undefined,
    codecString: f.codecString ?? undefined,
    isDefault: f.isDefault,
    width: f.width,
    height: f.height,
    channels: f.channels,
    sampleRate: f.sampleRate,
  }
}

export async function describeVideoTrack(t: InputVideoTrack, index = 0): Promise<MediaTrackInfo> {
  const [languageCode, name, codec, codecString, disposition, width, height] = await Promise.all([
    t.getLanguageCode(), t.getName(), t.getCodec(), t.getCodecParameterString(), t.getDisposition(),
    t.getDisplayWidth(), t.getDisplayHeight(),
  ])
  return toTrackInfo({
    id: t.id, type: 'video', languageCode, name, codec, codecString,
    isDefault: disposition.default, width, height,
  }, index)
}

export async function describeAudioTrack(t: InputAudioTrack, index: number): Promise<MediaTrackInfo> {
  const [languageCode, name, codec, codecString, disposition, channels, sampleRate] = await Promise.all([
    t.getLanguageCode(), t.getName(), t.getCodec(), t.getCodecParameterString(), t.getDisposition(),
    t.getNumberOfChannels(), t.getSampleRate(),
  ])
  return toTrackInfo({
    id: t.id, type: 'audio', languageCode, name, codec, codecString,
    isDefault: disposition.default, channels, sampleRate,
  }, index)
}
