package controllers

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	common "github.com/patrickdk77/aws-s3-proxy/internal/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStoredGzipObjectNotRecompressed(t *testing.T) {
	m := setupMock(t, 0)
	config.Config.ContentEncoding = true
	stored := "\x1f\x8b\x08\x00 already gzip"
	o := object(stored)
	o.ContentEncoding = aws.String("gzip")
	o.ContentType = aws.String("application/javascript")
	m.On("S3get", mock.Anything, "bucket", "/a.js", noRange).
		Return(o, nil)
	req := httptest.NewRequest("GET", "/a.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	common.WrapHandler(AwsS3).ServeHTTP(rr, req)
	assert.Equal(t, []string{"gzip"},
		rr.Header().Values("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", rr.Header().Get("Vary"))
	assert.Equal(t, strconv.Itoa(len(stored)),
		rr.Header().Get("Content-Length"))
	assert.Equal(t, stored, rr.Body.String())
}
