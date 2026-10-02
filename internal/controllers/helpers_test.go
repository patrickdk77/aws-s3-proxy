package controllers

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/service"
	"github.com/stretchr/testify/mock"
)

var (
	noRange = mock.MatchedBy(func(s *string) bool {
		return s == nil
	})
	anyRange = mock.MatchedBy(func(s *string) bool {
		return s != nil
	})
)

func setupMock(t *testing.T, cacheSize int64) *MockAWS {
	t.Helper()
	old := *config.Config
	oldClient := NewClientFunc
	t.Cleanup(func() {
		*config.Config = old
		NewClientFunc = oldClient
		httpCache = nil
		cacheOnce = sync.Once{}
	})
	config.Config.S3Bucket = "bucket"
	config.Config.S3KeyPrefix = ""
	config.Config.CacheSize = cacheSize
	config.Config.CacheTTL = time.Minute
	config.Config.CacheTTLIndex = time.Minute
	config.Config.CacheMaxFileSize = 1 << 20
	m := new(MockAWS)
	NewClientFunc = func(context.Context, *string) service.AWS {
		return m
	}
	httpCache = nil
	cacheOnce = sync.Once{}
	return m
}

func object(body string) *s3.GetObjectOutput {
	r := bytes.NewBufferString(body)
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(r),
		ContentLength: aws.Int64(int64(len(body))),
		ContentType:   aws.String("text/plain"),
	}
}
