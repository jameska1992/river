import { describe, expect, it } from 'vitest'
import { channelLabel, describeAudioTrack, describeVideoTrack, languageLabel, toTrackInfo, type RawTrackFacts } from './MediaTrackAdapter'
import type { InputAudioTrack, InputVideoTrack } from 'mediabunny'

const base: RawTrackFacts = {
  id: 2, type: 'audio', languageCode: 'eng', name: null, codec: 'aac', codecString: 'mp4a.40.2', isDefault: true,
  channels: 6, sampleRate: 48000,
}

describe('languageLabel', () => {
  it('names ISO 639-1 and 639-2 codes', () => {
    expect(languageLabel('en')).toBe('English')
    expect(languageLabel('eng')).toBe('English')
    expect(languageLabel('fre')).toBe('French')
  })
  it('falls back to the code', () => {
    expect(languageLabel('qqq')).toBe('qqq')
    expect(languageLabel(undefined)).toBeUndefined()
  })
})

describe('channelLabel', () => {
  it('names common layouts', () => {
    expect(channelLabel(1)).toBe('Mono')
    expect(channelLabel(2)).toBe('Stereo')
    expect(channelLabel(6)).toBe('5.1')
    expect(channelLabel(8)).toBe('7.1')
    expect(channelLabel(4)).toBe('4ch')
    expect(channelLabel(undefined)).toBeUndefined()
  })
})

describe('toTrackInfo', () => {
  it('maps an audio track with a language-derived label and layout', () => {
    expect(toTrackInfo(base, 0)).toEqual({
      id: '2', type: 'audio', language: 'eng', label: 'English (5.1)', codec: 'aac', codecString: 'mp4a.40.2',
      isDefault: true, width: undefined, height: undefined, channels: 6, sampleRate: 48000,
    })
  })

  it('prefers the container track name', () => {
    expect(toTrackInfo({ ...base, name: 'Director commentary', channels: 2 }, 0).label).toBe('Director commentary (Stereo)')
  })

  it('does not duplicate a layout already in the name', () => {
    expect(toTrackInfo({ ...base, name: 'English 5.1' }, 0).label).toBe('English 5.1')
  })

  it('treats "und" as no language and numbers the track', () => {
    const t = toTrackInfo({ ...base, languageCode: 'und', channels: undefined }, 3)
    expect(t.language).toBeUndefined()
    expect(t.label).toBe('Track 4')
  })

  it('maps a video track without channel decoration', () => {
    const t = toTrackInfo({ id: 1, type: 'video', languageCode: 'und', name: null, codec: 'avc', codecString: 'avc1.640028', isDefault: true, width: 1920, height: 1080 }, 0)
    expect(t).toMatchObject({ id: '1', type: 'video', codec: 'avc', width: 1920, height: 1080, label: 'Track 1' })
  })
})

describe('describe*Track', () => {
  const common = {
    id: 7,
    getLanguageCode: async () => 'jpn',
    getName: async () => null,
    getDisposition: async () => ({ default: false }),
  }

  it('resolves Mediabunny video getters', async () => {
    const t = {
      ...common,
      getCodec: async () => 'avc', getCodecParameterString: async () => 'avc1.64001f',
      getDisplayWidth: async () => 1280, getDisplayHeight: async () => 720,
    } as unknown as InputVideoTrack
    expect(await describeVideoTrack(t)).toMatchObject({ id: '7', type: 'video', width: 1280, height: 720, language: 'jpn' })
  })

  it('resolves Mediabunny audio getters', async () => {
    const t = {
      ...common,
      getCodec: async () => 'aac', getCodecParameterString: async () => 'mp4a.40.2',
      getNumberOfChannels: async () => 2, getSampleRate: async () => 44100,
    } as unknown as InputAudioTrack
    expect(await describeAudioTrack(t, 1)).toMatchObject({ id: '7', label: 'Japanese (Stereo)', sampleRate: 44100, isDefault: false })
  })
})
