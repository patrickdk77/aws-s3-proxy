package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HealthCheck = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "healthcheck_http_requests_total",
		Help: "Health check response codes",
	}, []string{"status"})
)
