package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HealthCheck = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "health_check",
		Help: "Health check response codes",
	}, []string{"status"})
)
