package utils

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ReqCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "app_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "path", "status"},
	)
	ReqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "app_request_duration_seconds", Help: "Request duration seconds"},
		[]string{"method", "path"},
	)
	ErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "app_errors_total", Help: "Total app errors"},
		[]string{"handler", "type"},
	)
)

func InitMetrics() {
	prometheus.MustRegister(ReqCount, ReqDuration, ErrorCount)
}
