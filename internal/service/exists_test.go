package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func exists(t *testing.T, status int, length string) bool {
	t.Helper()
	offline(t)
	t.Setenv("AWS_MAX_ATTEMPTS", "1")
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", length)
		w.WriteHeader(status)
	}
	srv := httptest.NewServer(http.HandlerFunc(h))
	t.Cleanup(srv.Close)
	config.Config.AwsAPIEndpoint = srv.URL
	ctx := context.Background()
	c := NewClient(ctx, aws.String("us-east-1"))
	return c.S3exists(ctx, "bucket", "/d/index.html")
}

func TestS3existsPresent(t *testing.T) {
	assert.True(t, exists(t, http.StatusOK, "120"))
}

func TestS3existsEmptyObject(t *testing.T) {
	assert.True(t, exists(t, http.StatusOK, "0"))
}

func TestS3existsMissing(t *testing.T) {
	assert.False(t, exists(t, http.StatusNotFound, "0"))
}

func TestS3existsForbiddenNotMissing(t *testing.T) {
	assert.True(t, exists(t, http.StatusForbidden, "0"))
}

func TestS3existsServerErrorNotMissing(t *testing.T) {
	assert.True(t, exists(t, http.StatusServiceUnavailable, "0"))
}

func TestS3existsUnreachableNotMissing(t *testing.T) {
	offline(t)
	t.Setenv("AWS_MAX_ATTEMPTS", "1")
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	config.Config.AwsAPIEndpoint = srv.URL
	ctx := context.Background()
	c := NewClient(ctx, aws.String("us-east-1"))
	assert.True(t, c.S3exists(ctx, "bucket", "/x"))
}
