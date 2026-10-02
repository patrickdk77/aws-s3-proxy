package controllers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/service"
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
