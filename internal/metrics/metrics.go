package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aegisproxy_requests_total",
			Help: "Total number of intercepted requests",
		},
		[]string{"status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aegisproxy_request_duration_seconds",
			Help:    "Latency of the proxied requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	TokensMasked = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aegisproxy_tokens_masked_total",
			Help: "Total number of PII tokens masked",
		},
		[]string{"type"},
	)
)
