package controllers

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAPIError struct {
	code    string
	message string
	fault   smithy.ErrorFault
}

func (m mockAPIError) Error() string {
	return m.message
}
func (m mockAPIError) ErrorCode() string {
	return m.code
}
func (m mockAPIError) ErrorMessage() string {
	return m.message
}
func (m mockAPIError) ErrorFault() smithy.ErrorFault {
	return m.fault
}

func TestToHTTPError(t *testing.T) {
	expectedCode := http.StatusInternalServerError
	expectedMsg := "Internal Server Error"

	code, msg := toHTTPError(errors.New("test"))

	assert.Equal(t, expectedCode, code)
	assert.Equal(t, expectedMsg, msg)
}

func TestToHTTPNoSuchBucketError(t *testing.T) {
	expectedCode := http.StatusNotFound
	expectedMsg := "Not Found"

	// Using typed error
	err := &types.NoSuchBucket{Message: aws.String("gone")}

	code, msg := toHTTPError(err)
	assert.Equal(t, expectedCode, code)
	assert.Contains(t, msg, expectedMsg)
}

func TestToHTTPNoSuchKeyError(t *testing.T) {
	expectedCode := http.StatusNotFound
	expectedMsg := "Not Found"

	// Using API Error
	err := mockAPIError{code: "NoSuchKey", message: "NoSuchKey"}

	code, msg := toHTTPError(err)
	assert.Equal(t, expectedCode, code)
	assert.Equal(t, expectedMsg, msg)
}

func TestToHTTPInvalidRangeError(t *testing.T) {
	expectedCode := http.StatusRequestedRangeNotSatisfiable
	expectedMsg := "Requested Range Not Satisfiable"

	// Using API Error
	err := mockAPIError{code: "InvalidRange", message: "bad"}

	code, msg := toHTTPError(err)
	assert.Equal(t, expectedCode, code)
	assert.Equal(t, expectedMsg, msg)
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

const leaky = `operation error S3: GetObject, request send failed, ` +
	`Get "https://my-private-bucket.s3.us-east-1.amazonaws.com/` +
	`secret-prefix/report.pdf?x-id=GetObject": connection refused`

func TestToHTTPErrorHidesDetails(t *testing.T) {
	logs := captureLog(t)
	code, msg := toHTTPError(errors.New(leaky))
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "Internal Server Error", msg)
	assert.Contains(t, logs.String(), "my-private-bucket")
}

func TestToHTTPErrorNotFoundNotLogged(t *testing.T) {
	logs := captureLog(t)
	code, msg := toHTTPError(mockAPIError{code: "NoSuchKey",
		message: "secret-prefix/x"})
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "Not Found", msg)
	assert.Empty(t, logs.String())
}

func TestToHTTPAccessDenied(t *testing.T) {
	code, msg := toHTTPError(mockAPIError{code: "AccessDenied",
		message: "arn:aws:iam::123456789012:role/proxy"})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "Forbidden", msg)
}

func TestErrorResponseHidesDetails(t *testing.T) {
	captureLog(t)
	m := setupMock(t, 0)
	m.On("S3get", mock.Anything, "bucket", "/f", noRange).
		Return(nil, errors.New(leaky))
	m.On("S3head", mock.Anything, "bucket", "/f", noRange).
		Return(nil, errors.New(leaky))
	for _, method := range []string{"GET", "HEAD"} {
		rr := httptest.NewRecorder()
		AwsS3(rr, httptest.NewRequest(method, "/f", nil))
		assert.Equal(t, 500, rr.Code, method)
		assert.NotContains(t, rr.Body.String(), "my-private")
	}
}

func TestListingErrorHidesDetails(t *testing.T) {
	captureLog(t)
	m := setupMock(t, 0)
	config.Config.DirectoryListing = true
	m.On("S3listObjects", mock.Anything, "bucket", mock.Anything).
		Return(nil, errors.New(leaky))
	rr := httptest.NewRecorder()
	AwsS3(rr, httptest.NewRequest("GET", "/d/", nil))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.NotContains(t, rr.Body.String(), "my-private")
}
