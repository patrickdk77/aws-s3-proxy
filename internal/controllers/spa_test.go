package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var notFound = mockAPIError{code: "NoSuchKey", message: "missing"}

func spaSetup(t *testing.T) *MockAWS {
	t.Helper()
	m := setupMock(t, 0)
	config.Config.SPA = true
	config.Config.IndexDocument = "index.html"
	return m
}

func serve(method, path string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	AwsS3(rr, httptest.NewRequest(method, path, nil))
	return rr
}

func TestSPAStripPathWithoutSlash(t *testing.T) {
	m := spaSetup(t)
	config.Config.StripPath = "/app/"
	m.On("S3get", mock.Anything, "bucket", "missing", noRange).
		Return(nil, notFound)
	m.On("S3get", mock.Anything, "bucket", "index.html", noRange).
		Return(object("<app>"), nil)
	var rr *httptest.ResponseRecorder
	assert.NotPanics(t, func() {
		rr = serve("GET", "/app/missing")
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "<app>", rr.Body.String())
}

func TestSPAStripPathWithoutSlashHead(t *testing.T) {
	m := spaSetup(t)
	config.Config.StripPath = "/app/"
	m.On("S3head", mock.Anything, "bucket", "missing", noRange).
		Return(nil, notFound)
	m.On("S3head", mock.Anything, "bucket", "index.html",
		noRange).Return(&s3.HeadObjectOutput{
		ContentLength: aws.Int64(5),
	}, nil)
	var rr *httptest.ResponseRecorder
	assert.NotPanics(t, func() {
		rr = serve("HEAD", "/app/missing")
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "5", rr.Header().Get("Content-Length"))
}

func TestSPANestedFallback(t *testing.T) {
	m := spaSetup(t)
	m.On("S3get", mock.Anything, "bucket", "/a/b/x", noRange).
		Return(nil, notFound)
	m.On("S3get", mock.Anything, "bucket", "/a/b/index.html",
		noRange).Return(object("<b>"), nil)
	rr := serve("GET", "/a/b/x")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "<b>", rr.Body.String())
}

func TestSPAIndexAlsoMissing(t *testing.T) {
	m := spaSetup(t)
	m.On("S3get", mock.Anything, "bucket", "/a/x", noRange).
		Return(nil, notFound)
	m.On("S3get", mock.Anything, "bucket", "/a/index.html",
		noRange).Return(nil, notFound)
	rr := serve("GET", "/a/x")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSPANoFallbackOnServerError(t *testing.T) {
	captureLog(t)
	m := spaSetup(t)
	m.On("S3get", mock.Anything, "bucket", "/a/x", noRange).
		Return(nil, errors.New("timeout"))
	rr := serve("GET", "/a/x")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	m.AssertNumberOfCalls(t, "S3get", 1)
}

func TestSPADisabledReturns404(t *testing.T) {
	m := spaSetup(t)
	config.Config.SPA = false
	m.On("S3get", mock.Anything, "bucket", "/a/x", noRange).
		Return(nil, notFound)
	rr := serve("GET", "/a/x")
	assert.Equal(t, http.StatusNotFound, rr.Code)
	m.AssertNumberOfCalls(t, "S3get", 1)
}

func TestSPAIndexItselfMissing(t *testing.T) {
	m := spaSetup(t)
	m.On("S3get", mock.Anything, "bucket", "/a/index.html",
		noRange).Return(nil, notFound)
	rr := serve("GET", "/a/index.html")
	assert.Equal(t, http.StatusNotFound, rr.Code)
	m.AssertNumberOfCalls(t, "S3get", 1)
}
