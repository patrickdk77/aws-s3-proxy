package metrics

import (
	"errors"

	"github.com/aws/smithy-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	GetObjectAction     = "GetObject"
	ListObjectAction    = "ListObject"
	UnknownS3Error      = "UnknownS3Error"
	DefaultResponseCode = "OK"
	HealthcheckSource   = "healthcheck"
	ProxySource         = "proxy"
)

var (
	HealthCheck = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "healthcheck_http_requests_total",
		Help: "Health check response codes",
	}, []string{"status"})
	S3Reads = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "s3_http_requests_total",
		Help: "s3 response codes",
	}, []string{"action", "responseCode", "source"})
)

/*
UpdateS3Reads receives the AWS error, action and source
and updates the s3_http_requests_total custom metric
*/
func UpdateS3Reads(err error, action, source string) {
	if err == nil {
		S3Reads.WithLabelValues(
			action,
			DefaultResponseCode,
			source,
		).Inc()
		return
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		S3Reads.WithLabelValues(
			action,
			ae.ErrorCode(),
			source,
		).Inc()
		return
	}
	S3Reads.WithLabelValues(
		action,
		UnknownS3Error,
		source,
	).Inc()
}
