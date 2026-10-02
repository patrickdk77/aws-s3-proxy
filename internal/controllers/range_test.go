package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDetermineHTTPStatus(t *testing.T) {
	for _, c := range []struct {
		rng    *string
		length *int64
		want   int
	}{
		{nil, aws.Int64(7), 200},
		{aws.String(""), aws.Int64(7), 200},
		{aws.String("bytes 0-1/7"), aws.Int64(2), 206},
		{aws.String("bytes 0-6/7"), aws.Int64(7), 200},
		{aws.String("bytes 0-1/*"), aws.Int64(2), 206},
		{aws.String("bytes 0-1/7"), nil, 206},
		{aws.String("garbage"), aws.Int64(7), 206},
	} {
		got := determineHTTPStatus(c.rng, c.length)
		assert.Equal(t, c.want, got, aws.ToString(c.rng))
	}
}

func head(rng string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("HEAD", "/f", nil)
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	rr := httptest.NewRecorder()
	AwsS3(rr, req)
	return rr
}

func TestHeadRangePartial(t *testing.T) {
	m := setupMock(t, 0)
	m.On("S3head", mock.Anything, "bucket", "/f", anyRange).
		Return(&s3.HeadObjectOutput{
			ContentLength: aws.Int64(2),
			ContentRange:  aws.String("bytes 0-1/7"),
		}, nil)
	rr := head("bytes=0-1")
	assert.Equal(t, http.StatusPartialContent, rr.Code)
	assert.Equal(t, "bytes 0-1/7",
		rr.Header().Get("Content-Range"))
}

func TestHeadRangeWholeObject(t *testing.T) {
	m := setupMock(t, 0)
	m.On("S3head", mock.Anything, "bucket", "/f", anyRange).
		Return(&s3.HeadObjectOutput{
			ContentLength: aws.Int64(7),
			ContentRange:  aws.String("bytes 0-6/7"),
		}, nil)
	assert.Equal(t, http.StatusOK, head("bytes=0-").Code)
}

func TestHeadNoRange(t *testing.T) {
	m := setupMock(t, 0)
	m.On("S3head", mock.Anything, "bucket", "/f", noRange).
		Return(&s3.HeadObjectOutput{
			ContentLength: aws.Int64(7),
		}, nil)
	rr := head("")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "7", rr.Header().Get("Content-Length"))
}

func TestGetRangeWithoutLength(t *testing.T) {
	m := setupMock(t, 0)
	o := object("co")
	o.ContentLength = nil
	o.ContentRange = aws.String("bytes 0-1/7")
	m.On("S3get", mock.Anything, "bucket", "/f", anyRange).
		Return(o, nil)
	var rr *httptest.ResponseRecorder
	assert.NotPanics(t, func() { rr = get("/f", "bytes=0-1") })
	assert.Equal(t, http.StatusPartialContent, rr.Code)
}
