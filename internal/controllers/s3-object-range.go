package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func determineHTTPStatus(contentRange *string, length *int64) int {
	cr := aws.ToString(contentRange)
	if cr == "" {
		return http.StatusOK
	}
	_, total, found := strings.Cut(cr, "/")
	total = strings.TrimSpace(total)
	size, err := strconv.ParseInt(total, 10, 64)
	if found && err == nil && size == aws.ToInt64(length) {
		return http.StatusOK
	}
	return http.StatusPartialContent
}
