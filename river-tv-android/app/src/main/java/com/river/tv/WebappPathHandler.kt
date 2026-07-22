package com.river.tv

import android.content.Context
import android.webkit.WebResourceResponse
import androidx.webkit.WebViewAssetLoader.PathHandler
import java.io.IOException

/**
 * Serves the bundled river-tv Vite build from `assets/webapp/`.
 *
 * If the requested path doesn't map to a real file *and* looks like
 * an SPA route (no file extension in the last segment, or empty),
 * we fall back to `index.html` so React Router's BrowserRouter can
 * handle deep links like `/movies/abc-123` on cold-start or reload.
 * Requests that clearly want a static asset (`foo.js`, `bar.png`)
 * return 404 instead so missing-file bugs surface loudly rather
 * than silently serving the HTML shell.
 */
class WebappPathHandler(private val context: Context) : PathHandler {

    override fun handle(path: String): WebResourceResponse? {
        val normalized = path.trimStart('/')
        openAsset(normalized)?.let { return it }

        val lastSegment = normalized.substringAfterLast('/', normalized)
        if (lastSegment.isEmpty() || !lastSegment.contains('.')) {
            return openAsset("index.html")
        }
        return null
    }

    private fun openAsset(relativePath: String): WebResourceResponse? {
        val assetPath = if (relativePath.isEmpty()) {
            "$ASSET_ROOT/index.html"
        } else {
            "$ASSET_ROOT/$relativePath"
        }
        return try {
            val stream = context.assets.open(assetPath)
            WebResourceResponse(mimeFor(assetPath), null, stream)
        } catch (_: IOException) {
            null
        }
    }

    private fun mimeFor(path: String): String = when (path.substringAfterLast('.').lowercase()) {
        "html", "htm" -> "text/html"
        "js", "mjs" -> "application/javascript"
        "css" -> "text/css"
        "json" -> "application/json"
        "map" -> "application/json"
        "svg" -> "image/svg+xml"
        "png" -> "image/png"
        "jpg", "jpeg" -> "image/jpeg"
        "gif" -> "image/gif"
        "webp" -> "image/webp"
        "avif" -> "image/avif"
        "ico" -> "image/x-icon"
        "woff" -> "font/woff"
        "woff2" -> "font/woff2"
        "ttf" -> "font/ttf"
        "otf" -> "font/otf"
        "txt" -> "text/plain"
        "webmanifest" -> "application/manifest+json"
        else -> "application/octet-stream"
    }

    companion object {
        private const val ASSET_ROOT = "webapp"
    }
}
