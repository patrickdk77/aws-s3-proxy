package http

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

const (
	ce = "Content-Encoding"
	cl = "Content-Length"
)

var page = strings.Repeat("<p>hello compressible world</p>", 50)

type writeFunc func(http.ResponseWriter)

func compressDo(t *testing.T, method, accept string,
	h writeFunc) *httptest.ResponseRecorder {
	t.Helper()
	restoreConfig(t)
	config.Config.ContentEncoding = true
	req := httptest.NewRequest(method, sample, nil)
	if accept != "" {
		req.Header.Set("Accept-Encoding", accept)
	}
	rr := httptest.NewRecorder()
	WrapHandler(func(w http.ResponseWriter, _ *http.Request) {
		h(w)
	}).ServeHTTP(rr, req)
	return rr
}

func html(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set(cl, "1550")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, page)
}

func typed(ct string) writeFunc {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", ct)
		_, _ = io.WriteString(w, page)
	}
}

func gunzip(t *testing.T, b []byte) string {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(b))
	if !assert.NoError(t, err) {
		return ""
	}
	out, err := io.ReadAll(r)
	assert.NoError(t, err)
	return string(out)
}

func TestCompressGzip(t *testing.T) {
	rr := compressDo(t, "GET", "gzip", html)
	h := rr.Header()
	assert.Equal(t, []string{"gzip"}, h.Values(ce))
	assert.Empty(t, h.Get(cl))
	assert.Equal(t, "Accept-Encoding", h.Get("Vary"))
	assert.Equal(t, page, gunzip(t, rr.Body.Bytes()))
}

func TestCompressDeflate(t *testing.T) {
	rr := compressDo(t, "GET", "br, deflate", html)
	assert.Equal(t, "deflate", rr.Header().Get(ce))
	r, err := zlib.NewReader(rr.Body)
	if assert.NoError(t, err) {
		out, _ := io.ReadAll(r)
		assert.Equal(t, page, string(out))
	}
}

func TestCompressIdentityWhenNotAccepted(t *testing.T) {
	for _, accept := range []string{"", "br", "gzip;q=0"} {
		rr := compressDo(t, "GET", accept, html)
		h := rr.Header()
		assert.Empty(t, h.Get(ce), accept)
		assert.Equal(t, "1550", h.Get(cl), accept)
		assert.Equal(t, "Accept-Encoding", h.Get("Vary"))
		assert.Equal(t, page, rr.Body.String(), accept)
	}
}

func TestCompressDisabled(t *testing.T) {
	restoreConfig(t)
	config.Config.ContentEncoding = false
	req := httptest.NewRequest("GET", sample, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	WrapHandler(func(w http.ResponseWriter, _ *http.Request) {
		html(w)
	}).ServeHTTP(rr, req)
	assert.Empty(t, rr.Header().Get(ce))
	assert.Empty(t, rr.Header().Get("Vary"))
	assert.Equal(t, page, rr.Body.String())
}

func TestCompressSkipsEncodedObject(t *testing.T) {
	var stored bytes.Buffer
	zw := gzip.NewWriter(&stored)
	_, _ = zw.Write([]byte(page))
	_ = zw.Close()
	h := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set(ce, "gzip")
		w.Header().Set(cl, "123")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(stored.Bytes())
	}
	rr := compressDo(t, "GET", "gzip", h)
	assert.Equal(t, []string{"gzip"}, rr.Header().Values(ce))
	assert.Equal(t, "123", rr.Header().Get(cl))
	assert.Equal(t, stored.Bytes(), rr.Body.Bytes())
}

func TestCompressSkipsPartialContent(t *testing.T) {
	h := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Range", "bytes 0-4/1550")
		w.Header().Set(cl, "5")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "<p>he")
	}
	rr := compressDo(t, "GET", "gzip", h)
	assert.Empty(t, rr.Header().Get(ce))
	assert.Equal(t, "5", rr.Header().Get(cl))
	assert.Equal(t, "<p>he", rr.Body.String())
}

func TestCompressSkipsBodylessStatus(t *testing.T) {
	for _, code := range []int{http.StatusNoContent,
		http.StatusNotModified} {
		h := func(w http.ResponseWriter) {
			w.WriteHeader(code)
		}
		rr := compressDo(t, "GET", "gzip", h)
		assert.Equal(t, code, rr.Code)
		assert.Empty(t, rr.Header().Get(ce))
		assert.Empty(t, rr.Body.Bytes())
	}
}

func TestCompressSkipsCompressedTypes(t *testing.T) {
	for _, ct := range []string{"image/png", "video/mp4",
		"audio/mpeg", "font/woff2", "application/zip",
		"application/gzip", "Image/JPEG; q=1"} {
		rr := compressDo(t, "GET", "gzip", typed(ct))
		assert.Empty(t, rr.Header().Get(ce), ct)
		assert.Equal(t, page, rr.Body.String(), ct)
	}
}

func TestCompressSvg(t *testing.T) {
	rr := compressDo(t, "GET", "gzip", typed("image/svg+xml"))
	assert.Equal(t, "gzip", rr.Header().Get(ce))
	assert.Equal(t, page, gunzip(t, rr.Body.Bytes()))
}

func TestCompressImplicitStatus(t *testing.T) {
	h := func(w http.ResponseWriter) {
		_, _ = io.WriteString(w, page)
	}
	rr := compressDo(t, "GET", "gzip", h)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "gzip", rr.Header().Get(ce))
	assert.Equal(t, "text/html; charset=utf-8",
		rr.Header().Get("Content-Type"))
	assert.Equal(t, page, gunzip(t, rr.Body.Bytes()))
}

func TestCompressErrorResponse(t *testing.T) {
	h := func(w http.ResponseWriter) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}
	rr := compressDo(t, "GET", "gzip", h)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "Not Found\n", gunzip(t, rr.Body.Bytes()))
}

func TestCompressHeadMatchesGet(t *testing.T) {
	headers := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set(cl, "1550")
		w.WriteHeader(http.StatusOK)
	}
	get := compressDo(t, "GET", "gzip", html)
	head := compressDo(t, "HEAD", "gzip", headers)
	assert.Equal(t, get.Header().Get(ce), head.Header().Get(ce))
	assert.Equal(t, get.Header().Get(cl), head.Header().Get(cl))
}

func TestCompressAccessLogCountsBytes(t *testing.T) {
	w := httptest.NewRecorder()
	c := &custom{Writer: w, ResponseWriter: w, encoding: "gzip"}
	_, _ = c.Write([]byte(page))
	assert.NoError(t, c.Close())
	assert.Equal(t, int64(len(page)), c.Written)
	assert.Less(t, w.Body.Len(), len(page))
}
