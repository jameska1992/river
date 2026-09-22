package subdl

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToVTT_ConvertsSRT(t *testing.T) {
	srt := "\ufeff1\r\n00:00:01,000 --> 00:00:04,500\r\nHello world\r\n"
	out := string(ToVTT([]byte(srt), "sub.srt"))

	assert.True(t, strings.HasPrefix(out, "WEBVTT\n\n"), "VTT header prepended")
	assert.Contains(t, out, "00:00:01.000 --> 00:00:04.500", "comma separators become dots")
	assert.NotContains(t, out, "\ufeff", "BOM stripped")
	assert.NotContains(t, out, "\r", "CRLF normalised")
}

func TestToVTT_PassesThroughVTT(t *testing.T) {
	vtt := "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHi\n"
	assert.Equal(t, vtt, string(ToVTT([]byte(vtt), "sub.vtt")))
}

func TestSearch(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"subtitles":[
			{"release_name":"Movie.2020.1080p","lang":"EN","language":"English","author":"bob","url":"/subtitle/1-2.zip"}
		]}`))
	}))
	defer srv.Close()

	c := &Client{http: srv.Client(), apiBase: srv.URL, dlHost: srv.URL}
	subs, err := c.Search(context.Background(), SearchParams{APIKey: "k", TmdbID: 27205, Type: "movie", Languages: "EN"})
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, "/subtitle/1-2.zip", subs[0].URL)
	assert.Equal(t, "English", subs[0].Language)

	// Query carried the key params.
	assert.Contains(t, gotQuery, "api_key=k")
	assert.Contains(t, gotQuery, "tmdb_id=27205")
	assert.Contains(t, gotQuery, "type=movie")
	assert.Contains(t, gotQuery, "languages=EN")
}

func TestSearch_NoResultsIsEmptyNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":false,"error":"No results found","subtitles":[]}`))
	}))
	defer srv.Close()

	c := &Client{http: srv.Client(), apiBase: srv.URL, dlHost: srv.URL}
	subs, err := c.Search(context.Background(), SearchParams{APIKey: "k", TmdbID: 1, Type: "movie"})
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestSearch_APIErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":false,"error":"Invalid API key"}`))
	}))
	defer srv.Close()

	c := &Client{http: srv.Client(), apiBase: srv.URL, dlHost: srv.URL}
	_, err := c.Search(context.Background(), SearchParams{APIKey: "bad", TmdbID: 1, Type: "movie"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid API key")
}

func TestDownload_ExtractsAndConvertsSRTFromZip(t *testing.T) {
	// Build a zip containing one .srt entry.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("Movie.srt")
	require.NoError(t, err)
	_, _ = f.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"))
	require.NoError(t, zw.Close())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/subtitle/1-2.zip", r.URL.Path)
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	c := &Client{http: srv.Client(), apiBase: srv.URL, dlHost: srv.URL}
	out, err := c.Download(context.Background(), "/subtitle/1-2.zip")
	require.NoError(t, err)
	s := string(out)
	assert.True(t, strings.HasPrefix(s, "WEBVTT"), "converted to VTT")
	assert.Contains(t, s, "00:00:01.000 --> 00:00:02.000")
}

func TestDownload_RejectsNonRootedURL(t *testing.T) {
	c := NewClient()
	_, err := c.Download(context.Background(), "https://evil.example.com/x.zip")
	require.Error(t, err, "an absolute/off-host url must be rejected (SSRF guard)")
}
