import { describe, expect, it } from 'vitest'
import { needsAacChannelRemap, remapAac51 } from './channelOrder'

/** Minimal AudioBuffer stand-in (jsdom has no Web Audio): channel i is filled with i. */
function fakeBuffer(channels: number, length = 4): AudioBuffer {
  const data = Array.from({ length: channels }, (_, i) => new Float32Array(length).fill(i))
  return {
    numberOfChannels: channels,
    length,
    getChannelData: (ch: number) => data[ch],
    copyToChannel: (src: Float32Array, ch: number) => data[ch].set(src),
  } as unknown as AudioBuffer
}
const channelValues = (b: AudioBuffer) => Array.from({ length: b.numberOfChannels }, (_, i) => b.getChannelData(i)[0])

describe('needsAacChannelRemap', () => {
  it.each([
    ['macOS Safari', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.4 Safari/605.1.15', true],
    ['iOS Safari', 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.4 Mobile/15E148 Safari/604.1', true],
    ['iOS Chrome (WebKit)', 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/135.0.7049.83 Mobile/15E148 Safari/604.1', true],
    ['desktop Chrome', 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36', false],
    ['Edge', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36 Edg/153.0.0.0', false],
    ['Android Chrome', 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Mobile Safari/537.36', false],
    ['Firefox', 'Mozilla/5.0 (X11; Linux x86_64; rv:155.0) Gecko/20100101 Firefox/155.0', false],
  ])('%s → %s', (_, ua, expected) => {
    expect(needsAacChannelRemap(ua)).toBe(expected)
  })
})

describe('remapAac51', () => {
  it('reorders C,L,R,Ls,Rs,LFE into L,R,C,LFE,SL,SR', () => {
    const b = fakeBuffer(6)
    expect(remapAac51(b)).toBe(true)
    // Decoded indices: 0=C 1=L 2=R 3=Ls 4=Rs 5=LFE
    expect(channelValues(b)).toEqual([1, 2, 0, 5, 3, 4])
  })

  it.each([1, 2, 8])('leaves %i-channel buffers alone', n => {
    const b = fakeBuffer(n)
    expect(remapAac51(b)).toBe(false)
    expect(channelValues(b)).toEqual(Array.from({ length: n }, (_, i) => i))
  })
})
