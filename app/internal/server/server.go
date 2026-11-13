// Package server wires HTTP routes, middleware and handlers for the public API.
package server

import (
	"log/slog"
	"net/http"

	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/items"
	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/metrics"
)

// Deps are the collaborators the handler tree needs.
type Deps struct {
	Logger    *slog.Logger
	Items     *items.Service
	Metrics   *metrics.Metrics
	Readiness *Readiness
	Version   string
	MaxPage   int
	// APIToken, when non-empty, is required on write endpoints.
	APIToken string
}

// NewHandler returns the public API handler. Health endpoints are unauthenticated
// by design: the ALB and ECS need to reach them without credentials.
func NewHandler(d Deps) http.Handler {
	h := &handlers{logger: d.Logger, items: d.Items, readiness: d.Readiness, version: d.Version, maxPage: d.MaxPage}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /readyz", h.readyz)
	mux.HandleFunc("GET /api/v1/items", h.listItems)
	mux.HandleFunc("GET /api/v1/items/{id}", h.getItem)

	var create http.Handler = http.HandlerFunc(h.createItem)
	if d.APIToken != "" {
		create = requireBearer(d.APIToken)(create)
	}
	mux.Handle("POST /api/v1/items", create)

	return chain(mux,
		requestID,
		accessLog(d.Logger, d.Metrics, mux),
		recoverer(d.Logger),
		securityHeaders,
	)
}

// NewMetricsHandler serves Prometheus metrics on a port that is not exposed
// through the ALB.
func NewMetricsHandler(m *metrics.Metrics) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", m.Handler())
	return mux
}
