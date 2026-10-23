/**
 * WebKit's AudioDecoder hands 5.1 AAC out in AAC's native channel order
 * (C, L, R, Ls, Rs, LFE), while Web Audio interprets a 6-channel AudioBuffer
 * as L, R, C, LFE, SL, SR. Chromium and Firefox reorder inside the decoder;
 * Safari doesn't, so without a remap the centre (dialogue) channel comes out
 * of front-left only and the rest of the mix is shuffled.
 *
 * For each Web Audio output channel, the decoded channel to take it from.
 */
const AAC_51_TO_WEBAUDIO = [1, 2, 0, 5, 3, 4] as const

/**
 * Whether decoded multichannel audio needs reordering on this browser.
 * Every iOS browser is WebKit underneath (CriOS, FxiOS, EdgiOS), so this
 * matches "AppleWebKit but not Blink".
 */
export function needsAacChannelRemap(ua: string = globalThis.navigator?.userAgent ?? ''): boolean {
  return /AppleWebKit/.test(ua) && !/Chrome|Chromium|Android/.test(ua)
}

/**
 * Reorders a decoded 5.1 buffer from AAC to Web Audio channel order, in
 * place. Buffers with any other channel count are left untouched.
 * Returns whether the buffer was reordered.
 */
export function remapAac51(buffer: AudioBuffer): boolean {
  if (buffer.numberOfChannels !== AAC_51_TO_WEBAUDIO.length) return false
  const decoded = AAC_51_TO_WEBAUDIO.map((_, ch) => buffer.getChannelData(ch).slice())
  AAC_51_TO_WEBAUDIO.forEach((from, to) => buffer.copyToChannel(decoded[from], to))
  return true
}
