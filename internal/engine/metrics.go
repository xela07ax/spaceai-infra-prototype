package engine

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	// Latency: сколько времени заняла обработка (включая коннекторы)
	RequestDuration *prometheus.HistogramVec

	// Traffic: общее кол-во запросов
	TotalRequests *prometheus.CounterVec

	// Errors: классификация отказов
	ErrorTotal *prometheus.CounterVec

	// Saturation: состояние Circuit Breaker (0 - ок, 1 - выбило)
	CircuitBreakerState *prometheus.GaugeVec

	// Audit: заполненность буфера (backpressure)
	AuditBufferFill prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	// Null Object Pattern - Если рег не передан, используем локальный, который никуда не подключен
	if reg == nil {
		reg = prometheus.NewRegistry()
	}

	return &Metrics{
		RequestDuration: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Name:    "uag_request_duration_seconds",
			Help:    "Histogram of request latencies.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"agent_id", "capability_id", "status"}),

		TotalRequests: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "uag_requests_total",
			Help: "Total number of processed requests.",
		}, []string{"agent_id", "capability_id"}),

		ErrorTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "uag_errors_total",
			Help: "Total number of errors by type.",
		}, []string{"type"}), // типы: policy_deny, blocked, rate_limit, timeout

		CircuitBreakerState: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "uag_circuit_breaker_state",
			Help: "Current state of the circuit breaker (0=closed, 1=open).",
		}, []string{"connector_id"}),

		AuditBufferFill: promauto.With(reg).NewGauge(prometheus.GaugeOpts{
			Name: "uag_audit_buffer_utilization",
			Help: "Current number of events in audit buffer.",
		}),
	}
}
