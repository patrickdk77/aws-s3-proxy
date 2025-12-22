package controllers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
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
	expectedMsg := "test"

	code, msg := toHTTPError(errors.New(expectedMsg))

	assert.Equal(t, expectedCode, code)
	assert.Equal(t, expectedMsg, msg)
}

func TestToHTTPNoSuchBucketError(t *testing.T) {
	expectedCode := http.StatusNotFound
	expectedMsg := "NoSuchBucket"

	// Using typed error
	err := &types.NoSuchBucket{Message: aws.String(expectedMsg)}

	code, msg := toHTTPError(err)
	assert.Equal(t, expectedCode, code)
	assert.Contains(t, msg, expectedMsg)
}

func TestToHTTPNoSuchKeyError(t *testing.T) {
	expectedCode := http.StatusNotFound
	expectedMsg := "NoSuchKey"

	// Using API Error
	err := mockAPIError{code: "NoSuchKey", message: expectedMsg}

	code, msg := toHTTPError(err)
	assert.Equal(t, expectedCode, code)
	assert.Equal(t, expectedMsg, msg)
}

func TestToHTTPInvalidRangeError(t *testing.T) {
	expectedCode := http.StatusRequestedRangeNotSatisfiable
	expectedMsg := "InvalidRange"

	// Using API Error
	err := mockAPIError{code: "InvalidRange", message: expectedMsg}

	code, msg := toHTTPError(err)
	assert.Equal(t, expectedCode, code)
	assert.Equal(t, expectedMsg, msg)
}
