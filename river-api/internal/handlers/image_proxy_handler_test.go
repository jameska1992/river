package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imageProxyRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/image", NewImageProxyHandler().Get)
	return r
}

func imageReq(rawURL string) string {
	return "/image?url=" + url.QueryEscape(rawURL)
}

func TestImageProxyHandler_Validation(t *testing.T) {
	r := imageProxyRouter()

	t.Run("missing url is 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodGet, "/image", "").Code)
	})
	t.Run("non-http scheme is 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodGet, imageReq("ftp://host/x.jpg"), "").Code)
	})
	t.Run("host not on allowlist is 403", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, doJSON(r, http.MethodGet, imageReq("https://evil.example.com/x.jpg"), "").Code)
	})
}

func TestImageProxyHandler_ProxiesAllowedHost(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("\xff\xd8\xff-jpeg-bytes"))
	}))
	defer upstream.Close()

	// The upstream is a 127.0.0.1 test server; allowlist it for the duration.
	host := hostOf(t, upstream.URL)
	allowedImageHosts[host] = struct{}{}
	defer delete(allowedImageHosts, host)

	w := doJSON(imageProxyRouter(), http.MethodGet, imageReq(upstream.URL+"/poster.jpg"), "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Cache-Control"), "immutable")
	assert.Contains(t, w.Body.String(), "jpeg-bytes")
}

func TestImageProxyHandler_ForwardsUpstreamStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()
	host := hostOf(t, upstream.URL)
	allowedImageHosts[host] = struct{}{}
	defer delete(allowedImageHosts, host)

	w := doJSON(imageProxyRouter(), http.MethodGet, imageReq(upstream.URL+"/gone.jpg"), "")
	assert.Equal(t, http.StatusNotFound, w.Code, "a stale poster's upstream 404 surfaces to the caller")
}

func TestImageProxyHandler_UpstreamUnreachableIs502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead := upstream.URL
	host := hostOf(t, dead)
	upstream.Close() // now nothing is listening

	allowedImageHosts[host] = struct{}{}
	defer delete(allowedImageHosts, host)

	w := doJSON(imageProxyRouter(), http.MethodGet, imageReq(dead+"/x.jpg"), "")
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func hostOf(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u.Hostname()
}
