package controllers

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

func toHTTPError(err error) (int, string) {
	if rerr, ok := err.(awserr.RequestFailure); ok {
		if rerr.StatusCode() == http.StatusRequestedRangeNotSatisfiable {
			return rerr.StatusCode(), rerr.Message()
		}
	}
	statusCode := http.StatusInternalServerError
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case s3.ErrCodeNoSuchBucket, s3.ErrCodeNoSuchKey:
			statusCode = http.StatusNotFound
		case "AccessDenied":
			statusCode = http.StatusForbidden
		}
	}
	return statusCode, err.Error()
}
