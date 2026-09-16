package com.river.mobile

import android.annotation.SuppressLint
import android.os.Bundle
import android.view.KeyEvent
import android.view.WindowManager
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.webkit.WebViewAssetLoader

/**
 * Single-Activity host that loads the bundled river-mobile Vite build into a
 * full-screen WebView. The phone counterpart to river-tv-android.
 *
 * The web bundle ships inside the APK (see the riverMobileSyncAssets Gradle
 * task in app/build.gradle.kts) and is served via WebViewAssetLoader under a
 * synthetic http://appassets.androidplatform.net/ origin. HTTP (not HTTPS) is
 * deliberate — river-api on a LAN is almost always plain HTTP, and matching
 * schemes sidesteps mixed-content blocking of <img>/<video> resources.
 * localStorage / Media Session / fetch all work fine on an HTTP origin.
 *
 * The API server URL is configured *inside* river-mobile (login screen), not
 * here — the default points at localhost, so on a real phone the user enters
 * their server's LAN address once.
 *
 * Back button: unlike the TV shell (which maps BACK → Escape for its D-pad
 * focus manager), the phone shell maps the hardware BACK to WebView history —
 * react-router pushes real history entries, so goBack() walks back through the
 * app (detail → list, exit the video route, etc.). At the root of history a
 * second BACK within two seconds exits.
 */
class MainActivity : AppCompatActivity() {
    private lateinit var webView: WebView
    private lateinit var assetLoader: WebViewAssetLoader
    private var lastBackPress = 0L

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Chrome remote-debugging for any debuggable build (chrome://inspect).
        if ((applicationInfo.flags and android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
            WebView.setWebContentsDebuggingEnabled(true)
        }

        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)

        assetLoader = WebViewAssetLoader.Builder()
            .setDomain(APP_ASSETS_DOMAIN)
            // Serve the bundled origin over HTTP so same-scheme HTTP river-api
            // calls aren't mixed content (see the class comment).
            .setHttpAllowed(true)
            .addPathHandler("/", WebappPathHandler(this))
            .build()

        webView = WebView(this).apply {
            layoutParams = FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT,
            )
            setBackgroundColor(0xFF000000.toInt())
            isFocusable = true
            isFocusableInTouchMode = true
            webViewClient = object : WebViewClient() {
                override fun shouldInterceptRequest(
                    view: WebView,
                    request: WebResourceRequest,
                ): WebResourceResponse? {
                    return assetLoader.shouldInterceptRequest(request.url)
                }
            }
            // Default WebChromeClient is enough; river-mobile's players draw
            // their own full-screen overlay rather than using the HTML5
            // Fullscreen API.
            webChromeClient = WebChromeClient()
            with(settings) {
                javaScriptEnabled = true
                domStorageEnabled = true
                // Let the audio/video players call play() programmatically
                // (playback is always kicked off by a tap first anyway).
                mediaPlaybackRequiresUserGesture = false

                // Honour the page's own responsive viewport
                // (width=device-width, viewport-fit=cover) — unlike the TV
                // shell we do NOT inject a fixed design width. Zoom is off for
                // a native app feel; textZoom pinned so the device font-size
                // setting doesn't rescale the fixed layout.
                useWideViewPort = true
                loadWithOverviewMode = false
                setSupportZoom(false)
                builtInZoomControls = false
                displayZoomControls = false
                textZoom = 100

                cacheMode = WebSettings.LOAD_DEFAULT
                mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
                allowFileAccess = false
                allowContentAccess = false
            }
        }
        setContentView(webView)
        webView.requestFocus()

        webView.loadUrl("http://$APP_ASSETS_DOMAIN/index.html")
    }

    override fun onKeyDown(keyCode: Int, event: KeyEvent?): Boolean {
        if (keyCode == KeyEvent.KEYCODE_BACK) {
            // Walk back through the SPA's history first; only when there's
            // nowhere left to go does BACK become the exit gesture.
            if (webView.canGoBack()) {
                webView.goBack()
                lastBackPress = 0L
                return true
            }
            val now = System.currentTimeMillis()
            if (now - lastBackPress < EXIT_WINDOW_MS) {
                finish()
                return true
            }
            lastBackPress = now
            Toast.makeText(this, R.string.press_back_again, Toast.LENGTH_SHORT).show()
            return true
        }
        return super.onKeyDown(keyCode, event)
    }

    override fun onPause() {
        webView.onPause()
        super.onPause()
    }

    override fun onResume() {
        super.onResume()
        webView.onResume()
    }

    override fun onDestroy() {
        webView.destroy()
        super.onDestroy()
    }

    companion object {
        private const val EXIT_WINDOW_MS = 2000L
        // The synthetic origin WebViewAssetLoader serves the bundled build from.
        private const val APP_ASSETS_DOMAIN = "appassets.androidplatform.net"
    }
}
