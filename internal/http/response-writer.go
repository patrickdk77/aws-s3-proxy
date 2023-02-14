package http

import (
	"io"
	"net/http"
)

type custom struct {
	io.Writer
	http.ResponseWriter
	status  int
	Written int64
}

func (c *custom) Write(b []byte) (int, error) {
	if c.Header().Get("Content-Type") == "" {
		c.Header().Set("Content-Type", http.DetectContentType(b))
	}
	n, err := c.Writer.Write(b)
	c.Written += int64(n)
	return n, err
}

func (c *custom) WriteHeader(status int) {
	c.ResponseWriter.WriteHeader(status)
	c.status = status
}
