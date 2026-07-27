package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resttrans_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "resttrans_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resttrans_http_errors_total",
			Help: "Total number of HTTP error responses (4xx/5xx)",
		},
		[]string{"method", "path", "status"},
	)

	InflightRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "resttrans_http_inflight_requests",
			Help: "Number of HTTP requests currently being processed",
		},
		[]string{"method"},
	)
)

func Register(reg prometheus.Registerer) {
	reg.MustRegister(
		RequestsTotal,
		RequestDuration,
		ErrorsTotal,
		InflightRequests,
	)
}
