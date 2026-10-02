package controllers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func get(path, rng string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	rr := httptest.NewRecorder()
	AwsS3(rr, req)
	return rr
}

func partial(body, rng string) *s3.GetObjectOutput {
	o := object(body)
	o.ContentRange = aws.String(rng)
	return o
}

func TestCacheRangeResponseNotStored(t *testing.T) {
	m := setupMock(t, 10<<20)
	m.On("S3get", mock.Anything, "bucket", "/init", noRange).
		Return(object("x"), nil).Once()
	m.On("S3get", mock.Anything, "bucket", "/f", anyRange).
		Return(partial("co", "bytes 0-1/7"), nil).Once()
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(object("content"), nil).Once()

	get("/init", "")
	assert.Equal(t, http.StatusPartialContent,
		get("/f", "bytes=0-1").Code)
	rr := get("/f", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "content", rr.Body.String())
	m.AssertExpectations(t)
}

func TestCacheRangeSkipsCachedCopy(t *testing.T) {
	m := setupMock(t, 10<<20)
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(object("content"), nil).Once()
	m.On("S3get", mock.Anything, "bucket", "/f", anyRange).
		Return(partial("nt", "bytes 2-3/7"), nil).Once()

	get("/f", "")
	rr := get("/f", "bytes=2-3")
	assert.Equal(t, http.StatusPartialContent, rr.Code)
	assert.Equal(t, "nt", rr.Body.String())
	assert.Equal(t, "bytes 2-3/7",
		rr.Header().Get("Content-Range"))
	m.AssertExpectations(t)
}

func TestCacheHeadRangeSkipsCachedCopy(t *testing.T) {
	m := setupMock(t, 10<<20)
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(object("content"), nil).Once()
	m.On("S3head", mock.Anything, "bucket", "/f", anyRange).
		Return(&s3.HeadObjectOutput{
			ContentLength: aws.Int64(2),
			ContentRange:  aws.String("bytes 0-1/7"),
		}, nil).Once()

	get("/f", "")
	req := httptest.NewRequest("HEAD", "/f", nil)
	req.Header.Set("Range", "bytes=0-1")
	rr := httptest.NewRecorder()
	AwsS3(rr, req)
	assert.Equal(t, "bytes 0-1/7",
		rr.Header().Get("Content-Range"))
	m.AssertExpectations(t)
}

func TestCacheConcurrentHitsFullBody(t *testing.T) {
	m := setupMock(t, 10<<20)
	body := strings.Repeat("a", 200000)
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(object(body), nil).Once()
	get("/f", "")

	var wg sync.WaitGroup
	var mu sync.Mutex
	short := 0
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if get("/f", "").Body.Len() != len(body) {
				mu.Lock()
				short++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 0, short)
}

func TestCacheSizeCountsBytes(t *testing.T) {
	m := setupMock(t, 1<<20)
	body := strings.Repeat("b", 300000)
	for i := 0; i < 5; i++ {
		p := fmt.Sprintf("/f%d", i)
		m.On("S3get", mock.Anything, "bucket", p, noRange).
			Return(object(body), nil).Once()
		get(p, "")
	}
	httpCache.SyncUpdates()
	assert.LessOrEqual(t, httpCache.GetSize(), int64(1<<20))
	assert.Less(t, httpCache.ItemCount(), 5)
	assert.Positive(t, httpCache.ItemCount())
}

func TestCacheEntrySize(t *testing.T) {
	r := cachedResponse{Body: make([]byte, 5000)}
	assert.Equal(t, int64(5000+cacheEntryOverhead), r.Size())
	assert.Equal(t, int64(cacheEntryOverhead),
		cachedResponse{}.Size())
}

func TestCacheHitHeadersMatchMiss(t *testing.T) {
	m := setupMock(t, 10<<20)
	mod := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	o := object("content")
	o.CacheControl = aws.String("public, max-age=60")
	o.ContentDisposition = aws.String("inline")
	o.ContentLanguage = aws.String("en")
	o.ContentType = aws.String("text/css")
	o.ETag = aws.String(`"abc"`)
	o.ExpiresString = aws.String("Thu, 01 Dec 2094 16:00:00 GMT")
	o.LastModified = &mod
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(o, nil).Once()

	miss := get("/f", "")
	hit := get("/f", "")
	assert.Equal(t, miss.Header(), hit.Header())
	assert.Equal(t, miss.Body.String(), hit.Body.String())
	m.AssertExpectations(t)
}

func TestCacheNotStoredWhenObjectForbids(t *testing.T) {
	for _, cc := range []string{"no-store", "No-Cache",
		"private, max-age=60"} {
		m := setupMock(t, 10<<20)
		o1, o2 := object("v1"), object("v2")
		o1.CacheControl = aws.String(cc)
		o2.CacheControl = aws.String(cc)
		m.On("S3get", mock.Anything, "bucket", "/f", noRange).
			Return(o1, nil).Once()
		m.On("S3get", mock.Anything, "bucket", "/f", noRange).
			Return(o2, nil).Once()
		get("/f", "")
		assert.Equal(t, "v2", get("/f", "").Body.String(), cc)
		m.AssertExpectations(t)
	}
}

func TestCacheStoredWithMaxAge(t *testing.T) {
	m := setupMock(t, 10<<20)
	o := object("v1")
	o.CacheControl = aws.String("public, max-age=60")
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(o, nil).Once()
	get("/f", "")
	assert.Equal(t, "v1", get("/f", "").Body.String())
	m.AssertExpectations(t)
}

func TestCacheBodyReadError(t *testing.T) {
	m := setupMock(t, 10<<20)
	o := object("")
	o.ContentLength = aws.Int64(100)
	o.Body = io.NopCloser(io.MultiReader(
		strings.NewReader("partial"),
		iotest.ErrReader(errors.New("connection reset"))))
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(o, nil).Once()
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(object("whole"), nil).Once()

	rr := get("/f", "")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.NotContains(t, rr.Body.String(), "partial")
	assert.Equal(t, "whole", get("/f", "").Body.String())
	m.AssertExpectations(t)
}

func TestCacheKeepsNoSDKMetadata(t *testing.T) {
	m := setupMock(t, 10<<20)
	o := object("content")
	o.ResultMetadata.Set("raw", "response")
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(o, nil).Once()
	get("/f", "")
	item := httpCache.Get("/f")
	if assert.NotNil(t, item) {
		md := item.Value().ResultMetadata
		assert.Nil(t, md.Get("raw"))
	}
}

func TestCacheTooLargeNotStored(t *testing.T) {
	m := setupMock(t, 10<<20)
	big := strings.Repeat("c", 2<<20)
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(object(big), nil).Twice()
	get("/f", "")
	get("/f", "")
	m.AssertExpectations(t)
}
