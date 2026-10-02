package http

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"
)

const (
	sampleSize = 128 << 10
	minSaving  = 5
)

type encoder interface {
	io.WriteCloser
	Flush() error
}

type switchWriter struct {
	io.Writer
}

type custom struct {
	io.Writer
	http.ResponseWriter
	status      int
	Written     int64
	encoding    string
	wroteHeader bool
	sampling    bool
	sample      []byte
	enc         encoder
}

func (c *custom) Write(b []byte) (int, error) {
	if c.Header().Get("Content-Type") == "" {
		c.Header().Set("Content-Type", http.DetectContentType(b))
	}
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if c.sampling {
		c.sample = append(c.sample, b...)
		c.Written += int64(len(b))
		if len(c.sample) < sampleSize {
			return len(b), nil
		}
		return len(b), c.decide()
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
	c.status = status
	if c.shouldCompress(status) {
		c.sampling = true
		return
	}
	c.ResponseWriter.WriteHeader(status)
}

func (c *custom) Close() error {
	if c.sampling {
		if err := c.decide(); err != nil {
			return err
		}
	}
	if c.enc == nil {
		return nil
	}
	return c.enc.Close()
}

func (c *custom) decide() error {
	c.sampling = false
	sample := c.sample
	c.sample = nil
	if len(sample) > 0 {
		var packed bytes.Buffer
		out := &switchWriter{Writer: &packed}
		enc := newEncoder(c.encoding, out)
		_, _ = enc.Write(sample)
		_ = enc.Flush()
		if saves(len(sample), packed.Len()) {
			c.Header().Set("Content-Encoding", c.encoding)
			c.Header().Del("Content-Length")
			c.ResponseWriter.WriteHeader(c.status)
			out.Writer = c.ResponseWriter
			c.Writer = enc
			c.enc = enc
			_, err := out.Write(packed.Bytes())
			return err
		}
		_ = enc.Close()
	}
	c.ResponseWriter.WriteHeader(c.status)
	if len(sample) == 0 {
		return nil
	}
	_, err := c.ResponseWriter.Write(sample)
	return err
}

func saves(raw, packed int) bool {
	return packed*100 <= raw*(100-minSaving)
}

func newEncoder(name string, w io.Writer) encoder {
	if name == "gzip" {
		return gzip.NewWriter(w)
	}
	return zlib.NewWriter(w)
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
