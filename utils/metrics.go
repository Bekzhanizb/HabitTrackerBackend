package utils

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Counter: Общее количество HTTP запросов
	ReqCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"}, // Labels (метки)
	)

	// Histogram: Время выполнения запросов
	ReqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "app_request_duration_seconds",
			Help: "Request duration seconds",
		},
		[]string{"method", "path"},
	)

	// Counter: Количество ошибок
	ErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_errors_total",
			Help: "Total app errors",
		},
		[]string{"handler", "type"}, // handler - какой endpoint, type - тип ошибки
	)
)

func InitMetrics() {
	// Регистрируем метрики в Prometheus
	prometheus.MustRegister(ReqCount, ReqDuration, ErrorCount)
}
