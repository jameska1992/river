package handlers

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ImageProxyHandler proxies image fetches from a small allowlist of
// external hosts (currently only the TMDB image CDN). The clients
// (river-tv WebView, river-web) call /image?url=<external-url> instead
// of hitting the CDN directly, so they never need cross-origin or
// arbitrary-host TLS reachability to render a poster.
//
// Why an explicit allowlist instead of an open proxy: an unrestricted
// /image?url=anything would be an SSRF foothold (an attacker could
// have river-api fetch internal-network URLs and stream the response
// back to them). We constrain hosts to known image CDNs.
type ImageProxyHandler struct {
	client *http.Client
}

// allowedHosts is checked exact-match against url.URL.Hostname().
// Add more entries here when you start storing artwork from a new CDN.
var allowedImageHosts = map[string]struct{}{
	"image.tmdb.org":         {}, // movie + TV posters/backdrops
	"covers.openlibrary.org": {}, // audiobook covers (river-meta-book)
}

func NewImageProxyHandler() *ImageProxyHandler {
	return &ImageProxyHandler{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get proxies a single external image URL.
//
// @Summary      Proxy an external image
// @Tags         image
// @Param        url   query  string  true  "Absolute https URL to proxy"
// @Produce      image/jpeg
// @Success      200
// @Failure      400  {object}  map[string]string  "missing or invalid url"
// @Failure      403  {object}  map[string]string  "host not on allowlist"
// @Failure      502  {object}  map[string]string  "upstream fetch failed"
// @Router       /image [get]
func (h *ImageProxyHandler) Get(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("url"))
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing url"})
		return
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	if _, ok := allowedImageHosts[u.Hostname()]; !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "host not allowed"})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, raw, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "build request failed"})
		return
	}
	// TMDB doesn't require any auth/UA but does 403 some browser UAs;
	// a neutral one keeps it happy.
	req.Header.Set("User-Agent", "river-api/1.0 (+image-proxy)")

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream fetch failed"})
		return
	}
	defer resp.Body.Close()

	// Bubble up the upstream content-type so the browser decodes
	// correctly. Forward the upstream status too so 404s from TMDB
	// surface as 404 to the caller (helpful when a poster ref is
	// stale and the UI should fall back to a placeholder).
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Header("Content-Length", cl)
	}
	// Cache aggressively. Posters/backdrops are immutable per URL.
	c.Header("Cache-Control", "public, max-age=604800, immutable")
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}
