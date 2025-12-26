package controllers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAWS is a mock type for the AWS interface
type MockAWS struct {
	mock.Mock
}

func (m *MockAWS) S3get(ctx context.Context, bucket, key string, rangeHeader *string) (*s3.GetObjectOutput, error) {
	args := m.Called(ctx, bucket, key, rangeHeader)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func (m *MockAWS) S3head(ctx context.Context, bucket, key string, rangeHeader *string) (*s3.HeadObjectOutput, error) {
	args := m.Called(ctx, bucket, key, rangeHeader)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.HeadObjectOutput), args.Error(1)
}

func (m *MockAWS) S3exists(ctx context.Context, bucket, key string) bool {
	args := m.Called(ctx, bucket, key)
	return args.Bool(0)
}

func (m *MockAWS) S3listObjects(ctx context.Context, bucket, prefix string) (*s3.ListObjectsV2Output, error) {
	args := m.Called(ctx, bucket, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.ListObjectsV2Output), args.Error(1)
}

func TestAwsS3_Caching(t *testing.T) {
	// Setup
	config.Config.AwsRegion = "us-east-1"
	config.Config.S3Bucket = "bucket"
	config.Config.S3KeyPrefix = ""
	config.Config.CacheSize = 10 * 1024 * 1024
	config.Config.CacheTTL = 1 * time.Minute
	config.Config.CacheMaxFileSize = 1 * 1024 * 1024

	mockAWS := new(MockAWS)
	NewClientFunc = func(ctx context.Context, region *string) service.AWS {
		return mockAWS
	}
	// Reset cache
	httpCache = nil
	cacheOnce = *new(sync.Once)

	// Test Case 1: Cache Miss
	mockAWS.On("S3get", mock.Anything, "bucket", "/file.txt", (*string)(nil)).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewBufferString("content")),
		ContentLength: aws.Int64(7),
		ContentType:   aws.String("text/plain"),
	}, nil).Once()

	req, _ := http.NewRequest("GET", "/file.txt", nil)
	rr := httptest.NewRecorder()

	AwsS3(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "content", rr.Body.String())
	mockAWS.AssertExpectations(t)

	// Test Case 2: Cache Hit
	// S3get should NOT be called again
	req, _ = http.NewRequest("GET", "/file.txt", nil)
	rr = httptest.NewRecorder()

	AwsS3(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "content", rr.Body.String())
	mockAWS.AssertExpectations(t)
}

func TestAwsS3_CacheMaxAge(t *testing.T) {
	// Setup
	config.Config.AwsRegion = "us-east-1"
	config.Config.S3Bucket = "bucket"
	config.Config.S3KeyPrefix = ""
	config.Config.CacheSize = 10 * 1024 * 1024
	config.Config.CacheTTL = 1 * time.Minute
	config.Config.CacheMaxFileSize = 1 * 1024 * 1024

	mockAWS := new(MockAWS)
	NewClientFunc = func(ctx context.Context, region *string) service.AWS {
		return mockAWS
	}
	// Reset cache
	httpCache = nil
	cacheOnce = *new(sync.Once)

	// Response with Cache-Control: max-age=1
	mockAWS.On("S3get", mock.Anything, "bucket", "/max-age.txt", (*string)(nil)).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewBufferString("expires quickly")),
		ContentLength: aws.Int64(15),
		ContentType:   aws.String("text/plain"),
		CacheControl:  aws.String("max-age=1"),
	}, nil).Once()

	// 1. Initial Request (Cache Miss)
	req, _ := http.NewRequest("GET", "/max-age.txt", nil)
	rr := httptest.NewRecorder()
	AwsS3(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "expires quickly", rr.Body.String())

	// 2. Immediate Request (Cache Hit)
	req, _ = http.NewRequest("GET", "/max-age.txt", nil)
	rr = httptest.NewRecorder()
	AwsS3(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "expires quickly", rr.Body.String())

	// 3. Wait for expiry
	time.Sleep(1100 * time.Millisecond)

	// 4. Request after expiry (Cache Miss again)
	mockAWS.On("S3get", mock.Anything, "bucket", "/max-age.txt", (*string)(nil)).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewBufferString("new content")),
		ContentLength: aws.Int64(11),
		ContentType:   aws.String("text/plain"),
	}, nil).Once()

	req, _ = http.NewRequest("GET", "/max-age.txt", nil)
	rr = httptest.NewRecorder()
	AwsS3(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "new content", rr.Body.String())
	mockAWS.AssertExpectations(t)
}

func TestAwsS3_CacheHEAD(t *testing.T) {
	// Setup
	config.Config.AwsRegion = "us-east-1"
	config.Config.S3Bucket = "bucket"
	config.Config.S3KeyPrefix = ""
	config.Config.CacheSize = 10 * 1024 * 1024
	config.Config.CacheTTL = 1 * time.Minute
	config.Config.CacheMaxFileSize = 1 * 1024 * 1024

	mockAWS := new(MockAWS)
	NewClientFunc = func(ctx context.Context, region *string) service.AWS {
		return mockAWS
	}
	// Reset cache
	httpCache = nil
	cacheOnce = *new(sync.Once)

	// 1. Initial GET Request (Cache Miss)
	mockAWS.On("S3get", mock.Anything, "bucket", "/head-cache.txt", (*string)(nil)).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewBufferString("head content")),
		ContentLength: aws.Int64(12),
		ContentType:   aws.String("text/plain"),
	}, nil).Once()

	req, _ := http.NewRequest("GET", "/head-cache.txt", nil)
	rr := httptest.NewRecorder()
	AwsS3(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "head content", rr.Body.String())

	// 2. HEAD Request (Cache Hit)
	// S3head should NOT be called. If it finds in cache, it uses cached GetObjectOutput.
	req, _ = http.NewRequest("HEAD", "/head-cache.txt", nil)
	rr = httptest.NewRecorder()
	AwsS3(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "12", rr.Header().Get("Content-Length")) // Verify header from cache
	assert.Equal(t, "", rr.Body.String())                    // HEAD has no body

	mockAWS.AssertExpectations(t)
}

func TestAwsS3_CacheMaxFileSize(t *testing.T) {
	// Setup
	config.Config.AwsRegion = "us-east-1"
	config.Config.S3Bucket = "bucket"
	config.Config.S3KeyPrefix = ""
	config.Config.CacheSize = 10 * 1024 * 1024
	config.Config.CacheTTL = 1 * time.Minute
	config.Config.CacheMaxFileSize = 1 * 1024 * 1024 // 10MB limit

	mockAWS := new(MockAWS)
	NewClientFunc = func(ctx context.Context, region *string) service.AWS {
		return mockAWS
	}
	// Reset cache
	httpCache = nil
	cacheOnce = *new(sync.Once)

	// Create large content > 10MB
	// ContentLength is what matters
	largeSize := int64(10*1024*1024 + 1)

	// 1. Initial Request (Should not cache)
	mockAWS.On("S3get", mock.Anything, "bucket", "/large.txt", (*string)(nil)).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader([]byte("small body"))),
		ContentLength: aws.Int64(largeSize),
		ContentType:   aws.String("text/plain"),
	}, nil).Once()

	req, _ := http.NewRequest("GET", "/large.txt", nil)
	rr := httptest.NewRecorder()
	AwsS3(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// 2. Second Request (Should trigger S3get again because it wasn't cached)
	mockAWS.On("S3get", mock.Anything, "bucket", "/large.txt", (*string)(nil)).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader([]byte("small body"))),
		ContentLength: aws.Int64(largeSize),
		ContentType:   aws.String("text/plain"),
	}, nil).Once()

	req, _ = http.NewRequest("GET", "/large.txt", nil)
	rr = httptest.NewRecorder()
	AwsS3(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	mockAWS.AssertExpectations(t)
}
