package controllers

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

func toHTTPError(err error) (int, string) {
	if rerr, ok := err.(awserr.RequestFailure); ok {
		switch rerr.StatusCode() {
		case http.StatusRequestedRangeNotSatisfiable:
			return rerr.StatusCode(), rerr.Message()
		}
	}
	statusCode := http.StatusInternalServerError
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case s3.ErrCodeNoSuchKey:
			statusCode = http.StatusNotFound
			break
		}
	}
	return statusCode, err.Error()
}
