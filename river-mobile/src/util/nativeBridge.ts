// Bridge to the river-mobile-android shell. The shell injects a `RiverNative`
// object (an Android @JavascriptInterface) onto window; in a plain browser it's
// absent and every call here is a no-op, so the web app behaves identically
// with or without the native host.
//
// Direction JS → native: keep a foreground service alive while audio is loaded
// (so playback survives backgrounding / screen-off) and mirror the Media
// Session metadata + play/pause state into the OS media notification.
//
// Direction native → JS: the shell drives playback from the notification /
// lock-screen by calling the controls we register on window.__riverAudio.

interface RiverNativeInterface {
  audioActive(active: boolean): void
  setMetadata(title: string, artist: string, album: string, durationMs: number): void
  setPlayback(playing: boolean, positionMs: number): void
}

export interface NativeAudioControls {
  play(): void
  pause(): void
  next(): void
  prev(): void
  // Absolute seek, in milliseconds (matches Android's MediaSession units).
  seekTo(ms: number): void
}

declare global {
  interface Window {
    RiverNative?: RiverNativeInterface
    __riverAudio?: NativeAudioControls
  }
}

function bridge(): RiverNativeInterface | undefined {
  return typeof window !== 'undefined' ? window.RiverNative : undefined
}

export function hasNativeAudio(): boolean {
  return !!bridge()
}

// Start (true) / stop (false) the native foreground service that keeps the
// WebView alive for background playback.
export function nativeAudioActive(active: boolean): void {
  bridge()?.audioActive(active)
}

export function nativeSetMetadata(title: string, artist: string, album: string, durationMs: number): void {
  bridge()?.setMetadata(title, artist, album, Math.max(0, Math.round(durationMs) || 0))
}

export function nativeSetPlayback(playing: boolean, positionMs: number): void {
  bridge()?.setPlayback(playing, Math.max(0, Math.round(positionMs) || 0))
}

// Expose the transport controls the native notification / lock-screen invoke.
// Returns a cleanup that removes them (if still ours).
export function registerNativeControls(controls: NativeAudioControls): () => void {
  window.__riverAudio = controls
  return () => { if (window.__riverAudio === controls) delete window.__riverAudio }
}
