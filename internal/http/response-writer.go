package http

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"
)

type custom struct {
	io.Writer
	http.ResponseWriter
	status      int
	Written     int64
	encoding    string
	wroteHeader bool
	closer      io.Closer
}

func (c *custom) Write(b []byte) (int, error) {
	if c.Header().Get("Content-Type") == "" {
		c.Header().Set("Content-Type", http.DetectContentType(b))
	}
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	n, err := c.Writer.Write(b)
	c.Written += int64(n)
	return n, err
}

func (c *custom) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true
	if c.shouldCompress(status) {
		c.Header().Set("Content-Encoding", c.encoding)
		c.Header().Del("Content-Length")
		var zw io.WriteCloser
		if c.encoding == "gzip" {
			zw = gzip.NewWriter(c.ResponseWriter)
		} else {
			zw = zlib.NewWriter(c.ResponseWriter)
		}
		c.Writer = zw
		c.closer = zw
	}
	c.ResponseWriter.WriteHeader(status)
	c.status = status
}

func (c *custom) Close() error {
	if c.closer == nil {
		return nil
	}
	return c.closer.Close()
}

func (c *custom) shouldCompress(status int) bool {
	h := c.Header()
	switch {
	case c.encoding == "",
		status < http.StatusOK,
		status == http.StatusNoContent,
		status == http.StatusPartialContent,
		status == http.StatusNotModified,
		h.Get("Content-Encoding") != "",
		h.Get("Content-Range") != "":
		return false
	}
	return compressible(h.Get("Content-Type"))
}

func compressible(contentType string) bool {
	ct, _, _ := strings.Cut(contentType, ";")
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "image/svg+xml" {
		return true
	}
	for _, p := range []string{"image/", "video/", "audio/",
		"font/woff"} {
		if strings.HasPrefix(ct, p) {
			return false
		}
	}
	switch ct {
	case "application/zip", "application/gzip",
		"application/x-gzip", "application/zstd",
		"application/x-bzip2", "application/x-xz",
		"application/x-7z-compressed", "application/vnd.rar",
		"application/x-rar-compressed":
		return false
	}
	return true
}
