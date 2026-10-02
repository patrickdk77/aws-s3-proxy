package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/patrickdk77/aws-s3-proxy/internal/service"
	"github.com/stretchr/testify/assert"
)

type fakeAWS struct {
	err error
}

func (f *fakeAWS) S3get(context.Context, string, string,
	*string) (*s3.GetObjectOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	body := io.NopCloser(bytes.NewReader([]byte("ok")))
	return &s3.GetObjectOutput{Body: body}, nil
}

func (f *fakeAWS) S3head(context.Context, string, string,
	*string) (*s3.HeadObjectOutput, error) {
	return nil, nil
}

func (f *fakeAWS) S3exists(context.Context, string, string) bool {
	return false
}

func (f *fakeAWS) S3listObjects(context.Context, string,
	string) (*s3.ListObjectsV2Output, error) {
	return nil, nil
}

func health(t *testing.T, f *fakeAWS) (int, HealthcheckResponse) {
	t.Helper()
	restoreConfig(t)
	old := newClient
	t.Cleanup(func() { newClient = old })
	newClient = func(context.Context, *string) service.AWS {
		return f
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	rr := httptest.NewRecorder()
	HealthcheckHandler(rr, httptest.NewRequest("GET", "/h", nil))
	var res HealthcheckResponse
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &res))
	return rr.Code, res
}

const leakyErr = `request send failed, Get "https://my-private-` +
	`bucket.s3.amazonaws.com/healthz": connection refused`

func TestHealthcheckHidesTransportError(t *testing.T) {
	code, res := health(t, &fakeAWS{err: errors.New(leakyErr)})
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.False(t, res.S3Bucket.Healthy)
	assert.Equal(t, "S3 request failed", res.S3Bucket.Error)
}

func TestHealthcheckShowsErrorCodeOnly(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "AccessDenied",
		Message: "arn:aws:iam::123456789012:role/x"}
	code, res := health(t, &fakeAWS{err: err})
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "AccessDenied", res.S3Bucket.Error)
}

func TestHealthcheckNoSuchKeyHealthy(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "NoSuchKey"}
	code, res := health(t, &fakeAWS{err: err})
	assert.Equal(t, http.StatusOK, code)
	assert.True(t, res.S3Bucket.Healthy)
	assert.Empty(t, res.S3Bucket.Error)
}
