package com.river.tv

import android.annotation.SuppressLint
import android.net.Uri
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
import androidx.appcompat.app.AppCompatActivity
import androidx.webkit.WebViewAssetLoader

/**
 * Single-Activity host that loads the bundled river-tv Vite build
 * into a full-screen WebView.
 *
 * The web bundle ships inside the APK (see the riverTvSyncAssets
 * Gradle task in app/build.gradle.kts) and is served via
 * WebViewAssetLoader under a synthetic http://appassets.android
 * platform.net/ origin. HTTP (not HTTPS) is deliberate — river-api
 * on a LAN is almost always plain HTTP, and matching schemes side-
 * steps mixed-content blocking of <img> / <video> resources on Fire
 * OS WebView builds. localStorage / IndexedDB / Fetch all work fine
 * on an HTTP origin; nothing in river-tv depends on a secure origin.
 *
 * The API server URL is configured *inside* river-tv (login screen),
 * not here.
 *
 * Remote handling:
 *  - Arrow keys + OK are delivered to the WebView as normal keydown
 *    events; river-tv's FocusProvider drives navigation from there.
 *  - The hardware BACK button is forwarded as an Escape keydown so
 *    river-tv's popup-close / player-exit / sidebar-focus logic fires.
 *    If the WebView truly has nowhere to go (root of router history),
 *    a second BACK press inside two seconds quits the app.
 */
class MainActivity : AppCompatActivity() {
    private lateinit var webView: WebView
    private lateinit var assetLoader: WebViewAssetLoader
    private var lastBackPress = 0L

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Enable Chrome remote-debugging for any debuggable build so a
        // developer machine can attach via chrome://inspect and use the
        // full DevTools (network, console, …) against the running app.
        // The flag is process-wide and only takes effect once.
        if ((applicationInfo.flags and android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
            WebView.setWebContentsDebuggingEnabled(true)
        }

        // Edge-to-edge feels right for TV — no system bars to worry about.
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        window.setFlags(
            WindowManager.LayoutParams.FLAG_FULLSCREEN,
            WindowManager.LayoutParams.FLAG_FULLSCREEN,
        )

        assetLoader = WebViewAssetLoader.Builder()
            .setDomain(APP_ASSETS_DOMAIN)
            // Serve the bundled origin over HTTP, not HTTPS. river-api on
            // a LAN is almost always plain HTTP, and an HTTPS page loading
            // HTTP <img> / <video> falls into mixed-content territory that
            // Fire TV WebView builds don't consistently honour the way
            // desktop Chrome does (MIXED_CONTENT_ALWAYS_ALLOW notwithstan-
            // ding — passive mixed content still blocks or silently gets
            // upgraded on some Fire OS builds). Matching schemes end-to-
            // end sidesteps the whole class of problem. No web API we use
            // requires a secure origin.
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

                override fun onPageFinished(view: WebView, url: String) {
                    super.onPageFinished(view, url)
                    applyDesignViewport(view)
                }
            }
            webChromeClient = WebChromeClient()
            with(settings) {
                javaScriptEnabled = true
                domStorageEnabled = true
                databaseEnabled = true
                // Don't gate <video autoplay> on a tap — TV users don't
                // tap. River-tv issues autoplay from JS as soon as the
                // player mounts.
                mediaPlaybackRequiresUserGesture = false

                // Zoom kill-switches. river-tv is laid out for a fixed
                // 1080p TV viewport.
                //  - useWideViewPort=true so the WebView honours the
                //    viewport meta we inject in applyDesignViewport()
                //    after each page load.
                //  - setSupportZoom / built-in zoom / display zoom all
                //    off so a stray gesture (or a connected mouse)
                //    can't change the scale at runtime.
                //  - textZoom forced to 100 so the device's font-size
                //    accessibility setting doesn't bump every rem.
                useWideViewPort = true
                loadWithOverviewMode = false
                setSupportZoom(false)
                builtInZoomControls = false
                displayZoomControls = false
                textZoom = 100

                cacheMode = WebSettings.LOAD_DEFAULT
                // The bundled origin is HTTP so same-scheme HTTP river-
                // api calls aren't mixed content. Kept permissive as a
                // safety net in case a stray HTTPS subresource ever gets
                // pulled in — better than a silent block.
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
            // Forward to the web app as Escape so its popup-close /
            // player-exit / detail-back handlers fire. The double-Back
            // exit gesture is only armed on pages the app treats as
            // roots (home + login) — every other page has its own JS
            // Back handler (e.g. browse pages navigate to /, detail
            // pages call history.back), and letting MainActivity's
            // timer fire on those would race the JS navigation and
            // exit the app while the user was still trying to move
            // one screen up.
            val now = System.currentTimeMillis()
            val onRoot = isRootUrl(webView.url)
            if (onRoot && now - lastBackPress < EXIT_WINDOW_MS) {
                finish()
                return true
            }
            // Only remember the timestamp when we're actually on a
            // root — otherwise a Back on a browse page followed by a
            // second Back at home (after navigation) would still meet
            // the two-in-2s test and exit unexpectedly.
            lastBackPress = if (onRoot) now else 0L
            // Dispatch on the currently-focused element (not window) so
            // a focused <input>'s own onKeyDown fires first and blurs
            // itself. If we dispatched on window, the FocusProvider
            // would see the Escape but the DOM input would keep focus
            // and swallow every subsequent D-pad press as text-cursor
            // movement. The event still bubbles to window afterwards,
            // so global back handlers (popup close, etc.) still fire
            // — unless a handler along the way calls stopPropagation,
            // which is exactly what FocusableInput does to keep Back
            // from also closing its parent popup on the same press.
            webView.evaluateJavascript(
                "(function(){var t=document.activeElement||document.body;" +
                    "t.dispatchEvent(new KeyboardEvent('keydown'," +
                    "{key:'Escape',bubbles:true,cancelable:true}));})()",
                null,
            )
            return true
        }
        return super.onKeyDown(keyCode, event)
    }

    /**
     * A "root" is a page where the double-Back exit gesture should be
     * armed — currently home ("/", "/index.html") and the login screen.
     * Every other page has a JS Back handler that navigates within the
     * app, so we don't want MainActivity's timer to race those and
     * quit while the user was mid-navigation.
     */
    private fun isRootUrl(url: String?): Boolean {
        if (url.isNullOrEmpty()) return true
        val path = try {
            Uri.parse(url).path.orEmpty()
        } catch (_: Exception) {
            return true
        }
        return path.isEmpty() || path == "/" || path == "/index.html" || path == "/login"
    }

    /**
     * Override the page's viewport meta so it lays out for the design's
     * 1920-CSS-px-wide canvas and renders at 1 CSS px = 1 physical
     * pixel, regardless of the device density.
     *
     * Without this, on a 1080p TV at density=2 the page's existing
     * `<meta viewport width=device-width>` reports a 960-CSS-px
     * layout, and Android's density scaling then renders each CSS px
     * at 2 physical px — visually a ~2× zoom compared to a 1920-wide
     * desktop browser.
     *
     * We patch the meta after page-load (the SPA only ever loads
     * once, so the visible flash happens once at startup).
     */
    private fun applyDesignViewport(view: WebView) {
        val dm = resources.displayMetrics
        // Pick an initial-scale that, combined with the density-driven
        // CSS-to-physical-pixel ratio, results in (DESIGN_WIDTH) CSS px
        // filling the entire physical screen width.
        val scale = dm.widthPixels.toDouble() / DESIGN_WIDTH / dm.density
        val js = """
            (function() {
                var m = document.querySelector('meta[name="viewport"]');
                if (!m) {
                    m = document.createElement('meta');
                    m.setAttribute('name', 'viewport');
                    document.head.appendChild(m);
                }
                m.setAttribute('content',
                    'width=$DESIGN_WIDTH, initial-scale=$scale, minimum-scale=$scale, maximum-scale=$scale, user-scalable=no'
                );
            })();
        """.trimIndent()
        view.evaluateJavascript(js, null)
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
        // CSS-pixel width river-tv is laid out for. Card grids, hero
        // height, font tokens etc. all assume this. Don't change unless
        // the web design itself moves to a different anchor.
        private const val DESIGN_WIDTH = 1920
        // The synthetic HTTPS origin WebViewAssetLoader serves the
        // bundled Vite build from.
        private const val APP_ASSETS_DOMAIN = "appassets.androidplatform.net"
    }
}
