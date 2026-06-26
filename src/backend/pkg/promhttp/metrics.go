package promhttp

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Register mounts GET /metrics on mux using the given registry (nil = default).
func Register(mux *http.ServeMux, registry *prometheus.Registry) {
	var handler http.Handler
	if registry != nil {
		handler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	} else {
		handler = promhttp.Handler()
	}
	mux.Handle("/metrics", handler)
}

// MountMetricsOnHealth registers /metrics on the same ServeMux returned by healthHandler().
func MountMetricsOnHealth(health http.Handler, registry *prometheus.Registry) http.Handler {
	if mux, ok := health.(*http.ServeMux); ok {
		Register(mux, registry)
		return mux
	}
	parent := http.NewServeMux()
	parent.Handle("/", health)
	Register(parent, registry)
	return parent
}

// MountHealthAndMetrics registers /health and /metrics on mux.
// health should be the handler for GET /health (not a nested mux).
func MountHealthAndMetrics(mux *http.ServeMux, health http.HandlerFunc, registry *prometheus.Registry) {
	mux.HandleFunc("/health", health)
	Register(mux, registry)
}
