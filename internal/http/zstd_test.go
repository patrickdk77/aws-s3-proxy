package http

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
)

func TestPickEncoding(t *testing.T) {
	for accept, want := range map[string]string{
		"":                           "",
		"gzip":                       "gzip",
		"deflate":                    "deflate",
		"deflate, gzip":              "gzip",
		"gzip, deflate, br, zstd":    "zstd",
		"zstd;q=0, gzip":             "gzip",
		"gzip;q=0":                   "",
		"gzip; q=0.000":              "",
		"gzip;q=0.5":                 "gzip",
		"GZIP":                       "gzip",
		"br":                         "",
		"*":                          "",
		"identity":                   "",
		"zstd;q=bogus, deflate":      "deflate",
		"gzip;q=0, gzip;q=1":         "gzip",
		" zstd ; q=0.1 , gzip;q=0.9": "zstd",
	} {
		assert.Equal(t, want, pickEncoding(accept), accept)
	}
}

func unzstd(t *testing.T, b []byte) string {
	t.Helper()
	d, err := zstd.NewReader(bytes.NewReader(b))
	if !assert.NoError(t, err) {
		return ""
	}
	defer d.Close()
	out, err := io.ReadAll(d)
	assert.NoError(t, err)
	return string(out)
}

func decode(t *testing.T, enc string, b []byte) string {
	t.Helper()
	switch enc {
	case "zstd":
		return unzstd(t, b)
	case "gzip":
		return gunzip(t, b)
	case "deflate":
		r, err := zlib.NewReader(bytes.NewReader(b))
		if !assert.NoError(t, err) {
			return ""
		}
		out, _ := io.ReadAll(r)
		return string(out)
	}
	return string(b)
}

const browser = "gzip, deflate, br, zstd"

func TestZstdSmallBody(t *testing.T) {
	rr := compressDo(t, "GET", browser, html)
	assert.Equal(t, "zstd", rr.Header().Get(ce))
	assert.Empty(t, rr.Header().Get(cl))
	assert.Equal(t, page, unzstd(t, rr.Body.Bytes()))
}

func TestZstdStreamsPastSample(t *testing.T) {
	body := []byte(strings.Repeat(page, 300))
	h := chunked("application/json", body)
	rr := compressDo(t, "GET", browser, h)
	assert.Equal(t, "zstd", rr.Header().Get(ce))
	assert.Equal(t, string(body), unzstd(t, rr.Body.Bytes()))
}

func TestZstdIncompressible(t *testing.T) {
	body := noise(200 << 10)
	h := chunked("application/octet-stream", body)
	rr := compressDo(t, "GET", browser, h)
	assert.Empty(t, rr.Header().Get(ce))
	assert.Equal(t, body, rr.Body.Bytes())
}

var names = []string{"zstd", "gzip", "deflate"}

func TestEncoderReuse(t *testing.T) {
	for _, enc := range names {
		for i := 0; i < 3; i++ {
			body := fmt.Sprintf("%d %s %s", i, enc, page)
			w := httptest.NewRecorder()
			c := &custom{Writer: w, ResponseWriter: w,
				encoding: enc}
			c.Header().Set("Content-Type", "text/plain")
			_, _ = c.Write([]byte(body))
			assert.NoError(t, c.Close())
			assert.Equal(t, enc, w.Header().Get(ce))
			got := decode(t, enc, w.Body.Bytes())
			assert.Equal(t, body, got)
		}
	}
}

func TestEncoderPoolConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			enc := names[i%3]
			item := fmt.Sprintf("<p>%d</p>", i)
			body := strings.Repeat(item, 20000)
			w := httptest.NewRecorder()
			c := &custom{Writer: w, ResponseWriter: w,
				encoding: enc}
			c.Header().Set("Content-Type", "text/html")
			for j := 0; j < len(body); j += 32 << 10 {
				end := min(j+32<<10, len(body))
				_, _ = c.Write([]byte(body[j:end]))
			}
			assert.NoError(t, c.Close())
			got := decode(t, enc, w.Body.Bytes())
			assert.Equal(t, body, got)
		}(i)
	}
	wg.Wait()
}

func TestGzipOutputReadableByStdlib(t *testing.T) {
	rr := compressDo(t, "GET", "gzip", html)
	r, err := gzip.NewReader(rr.Body)
	if assert.NoError(t, err) {
		out, _ := io.ReadAll(r)
		assert.Equal(t, page, string(out))
	}
}
