// Package metrics exposes Prometheus instrumentation for the HTTP service.
//
// Route labels use the ServeMux pattern (for example "GET /api/v1/items/{id}")
// rather than the raw path, which keeps label cardinality bounded.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics groups the collectors used by the HTTP middleware.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	latency  *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

// New creates a registry with Go runtime, process and HTTP collectors.
func New(serviceName, serviceVersion string) *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		registry: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total HTTP requests by method, route and status code.",
			ConstLabels: prometheus.Labels{"service": serviceName, "version": serviceVersion},
		}, []string{"method", "route", "code"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request latency in seconds by method and route.",
			Buckets:     []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			ConstLabels: prometheus.Labels{"service": serviceName, "version": serviceVersion},
		}, []string{"method", "route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "http_requests_in_flight",
			Help:        "Number of HTTP requests currently being served.",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}),
	}
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.requests, m.latency, m.inFlight,
	)
	return m
}

// Handler serves the Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// Observe records one completed request.
func (m *Metrics) Observe(method, route string, status int, elapsed time.Duration) {
	m.requests.WithLabelValues(method, route, statusClass(status)).Inc()
	m.latency.WithLabelValues(method, route).Observe(elapsed.Seconds())
}

// InFlight returns the gauge tracking concurrent requests.
func (m *Metrics) InFlight() prometheus.Gauge { return m.inFlight }

// statusClass collapses status codes to their class (2xx, 4xx...) to keep
// cardinality low while preserving the signal the 5xx alarm depends on.
func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}
