package http

import (
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func noise(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func chunked(ct string, body []byte) writeFunc {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", ct)
		w.Header().Set(cl, strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		for r := bytes.NewReader(body); r.Len() > 0; {
			_, _ = io.CopyN(w, r, 32<<10)
		}
	}
}

func TestSampleIncompressibleSentAsIs(t *testing.T) {
	body := noise(300 << 10)
	rr := compressDo(t, "GET", "gzip",
		chunked("application/octet-stream", body))
	assert.Empty(t, rr.Header().Get(ce))
	assert.Equal(t, strconv.Itoa(len(body)), rr.Header().Get(cl))
	assert.Equal(t, body, rr.Body.Bytes())
}

func TestSampleCompressibleStreams(t *testing.T) {
	body := []byte(strings.Repeat(page, 300))
	h := chunked("text/plain", body)
	rr := compressDo(t, "GET", "gzip", h)
	assert.Equal(t, "gzip", rr.Header().Get(ce))
	assert.Empty(t, rr.Header().Get(cl))
	assert.Less(t, rr.Body.Len(), len(body)/10)
	assert.Equal(t, string(body), gunzip(t, rr.Body.Bytes()))
}

func TestSampleDecidesOnFirstBlock(t *testing.T) {
	body := append(noise(sampleSize),
		[]byte(strings.Repeat(page, 200))...)
	h := chunked("text/plain", body)
	rr := compressDo(t, "GET", "gzip", h)
	assert.Empty(t, rr.Header().Get(ce))
	assert.Equal(t, body, rr.Body.Bytes())
}

func TestSampleSmallBodyDecidedAtClose(t *testing.T) {
	rr := compressDo(t, "GET", "gzip",
		chunked("text/plain", []byte(page)))
	assert.Equal(t, "gzip", rr.Header().Get(ce))
	assert.Equal(t, page, gunzip(t, rr.Body.Bytes()))
}

func TestSampleTinyBodyNotCompressed(t *testing.T) {
	rr := compressDo(t, "GET", "gzip",
		chunked("text/plain", []byte("ok")))
	assert.Empty(t, rr.Header().Get(ce))
	assert.Equal(t, "2", rr.Header().Get(cl))
	assert.Equal(t, "ok", rr.Body.String())
}

func TestSampleEmptyBody(t *testing.T) {
	h := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	}
	rr := compressDo(t, "GET", "gzip", h)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Header().Get(ce))
	assert.Empty(t, rr.Body.Bytes())
}

func TestSampleStatusKept(t *testing.T) {
	h := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, page)
	}
	rr := compressDo(t, "GET", "gzip", h)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "gzip", rr.Header().Get(ce))
}

func TestSampleWrittenCountsRawBytes(t *testing.T) {
	for _, body := range [][]byte{noise(200 << 10),
		[]byte(strings.Repeat(page, 200))} {
		w := httptest.NewRecorder()
		c := &custom{Writer: w, ResponseWriter: w,
			encoding: "gzip"}
		c.Header().Set("Content-Type", "text/plain")
		_, _ = c.Write(body)
		assert.NoError(t, c.Close())
		assert.Equal(t, int64(len(body)), c.Written)
	}
}

func TestSaves(t *testing.T) {
	assert.True(t, saves(100, 95))
	assert.False(t, saves(100, 96))
	assert.True(t, saves(1000, 10))
	assert.False(t, saves(1000, 1020))
}
