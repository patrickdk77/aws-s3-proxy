package controllers

import (
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

func toHTTPError(err error) (int, string) {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "NoSuchKey", "NoSuchBucket", "NotFound":
			return http.StatusNotFound, err.Error()
		case "AccessDenied":
			return http.StatusForbidden, err.Error()
		case "InvalidRange":
			return http.StatusRequestedRangeNotSatisfiable, err.Error()
		}
	}
	// Check for typed errors as fallback or specific handling
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return http.StatusNotFound, err.Error()
	}
	var nsb *types.NoSuchBucket
	if errors.As(err, &nsb) {
		return http.StatusNotFound, err.Error()
	}

	return http.StatusInternalServerError, err.Error()
}
