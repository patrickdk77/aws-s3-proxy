package controllers

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

func link(target string) (*s3.GetObjectOutput, *trackedBody) {
	body := &trackedBody{
		Reader: strings.NewReader(`{"URL":"` + target + `"}`),
	}
	return &s3.GetObjectOutput{Body: body}, body
}

func TestSymlinkSegmentAtEnd(t *testing.T) {
	m := setupMock(t, 0)
	obj, body := link("/b/target.txt")
	m.On("S3get", mock.Anything, "bucket", "/a/symlink.json",
		noRange).Return(obj, nil).Once()
	m.On("S3get", mock.Anything, "bucket", "/b/target.txt",
		noRange).Return(object("target"), nil).Once()
	rr := serve("GET", "/a/symlink.json")
	assert.Equal(t, "target", rr.Body.String())
	assert.True(t, body.closed)
	m.AssertExpectations(t)
}

func TestSymlinkSegmentInMiddle(t *testing.T) {
	m := setupMock(t, 0)
	obj, _ := link("/b")
	m.On("S3get", mock.Anything, "bucket", "/a/symlink.json",
		noRange).Return(obj, nil).Once()
	m.On("S3get", mock.Anything, "bucket", "/b/x.txt",
		noRange).Return(object("x"), nil).Once()
	rr := serve("GET", "/a/symlink.json/x.txt")
	assert.Equal(t, "x", rr.Body.String())
	m.AssertExpectations(t)
}

func TestSymlinkNameSuffixIsAFile(t *testing.T) {
	for _, p := range []string{"/a/mysymlink.json",
		"/a/symlink.jsonx", "/symlink.json.bak"} {
		m := setupMock(t, 0)
		m.On("S3get", mock.Anything, "bucket", p, noRange).
			Return(object("{}"), nil).Once()
		rr := serve("GET", p)
		assert.Equal(t, http.StatusOK, rr.Code, p)
		assert.Equal(t, "{}", rr.Body.String(), p)
		m.AssertExpectations(t)
	}
}

func TestSymlinkMissing(t *testing.T) {
	m := setupMock(t, 0)
	m.On("S3get", mock.Anything, "bucket", "/a/symlink.json",
		noRange).Return(nil, notFound).Once()
	rr := serve("GET", "/a/symlink.json/x")
	assert.Equal(t, http.StatusNotFound, rr.Code)
	m.AssertExpectations(t)
}

func TestSymlinkBadJSON(t *testing.T) {
	captureLog(t)
	m := setupMock(t, 0)
	body := &trackedBody{Reader: strings.NewReader("{not json")}
	obj := &s3.GetObjectOutput{Body: body}
	m.On("S3get", mock.Anything, "bucket", "/a/symlink.json",
		noRange).Return(obj, nil).Once()
	rr := serve("GET", "/a/symlink.json")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "Internal Server Error\n", rr.Body.String())
	assert.True(t, body.closed)
}

func TestSymlinkEnd(t *testing.T) {
	for path, want := range map[string]int{
		"/a/symlink.json":       15,
		"/a/symlink.json/x":     15,
		"symlink.json/x":        12,
		"/a/mysymlink.json":     -1,
		"/a/symlink.jsonx":      -1,
		"/a/b":                  -1,
		"/x/symlink.json/y/z/q": 15,
	} {
		assert.Equal(t, want, symlinkEnd(path), path)
	}
}
