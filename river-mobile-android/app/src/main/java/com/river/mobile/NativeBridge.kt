package com.river.mobile

import android.webkit.JavascriptInterface

/**
 * The `window.RiverNative` object injected into the WebView. river-mobile's
 * audio player calls these to (de)activate background playback and mirror its
 * Media Session state into the OS media notification.
 *
 * Methods run on the WebView's JS thread — they only hand off to [MainActivity]
 * (which marshals to the main thread / starts the service), so they stay cheap
 * and thread-safe. Numeric params are `Double` because JS numbers marshal to
 * Java doubles most reliably across WebView versions.
 */
class NativeBridge(private val activity: MainActivity) {

    @JavascriptInterface
    fun audioActive(active: Boolean) {
        if (active) activity.startAudioService() else activity.stopAudioService()
    }

    @JavascriptInterface
    fun setMetadata(title: String, artist: String, album: String, durationMs: Double) {
        activity.updateAudioMetadata(title, artist, album, durationMs.toLong())
    }

    @JavascriptInterface
    fun setPlayback(playing: Boolean, positionMs: Double) {
        activity.updateAudioPlayback(playing, positionMs.toLong())
    }
}
