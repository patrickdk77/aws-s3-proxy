package controllers

import (
	"errors"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

func toHTTPError(err error) (int, string) {
	code := http.StatusInternalServerError
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "NoSuchKey", "NoSuchBucket", "NotFound":
			code = http.StatusNotFound
		case "AccessDenied":
			code = http.StatusForbidden
		case "InvalidRange":
			code = http.StatusRequestedRangeNotSatisfiable
		}
	}
	// Check for typed errors as fallback or specific handling
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		code = http.StatusNotFound
	}
	var nsb *types.NoSuchBucket
	if errors.As(err, &nsb) {
		code = http.StatusNotFound
	}
	if code == http.StatusInternalServerError {
		log.Printf("[s3] %v", err)
	}
	return code, http.StatusText(code)
}
