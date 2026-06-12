package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type gatewayMetrics struct {
	mu                 sync.Mutex
	requestCount       map[string]int64
	requestLatencyMS   map[string]float64
	rateLimitHits      map[string]int64
	forceUpdateBlocks  map[string]int64
}

func newGatewayMetrics() *gatewayMetrics {
	return &gatewayMetrics{
		requestCount:      map[string]int64{},
		requestLatencyMS:  map[string]float64{},
		rateLimitHits:     map[string]int64{},
		forceUpdateBlocks: map[string]int64{},
	}
}

func (m *gatewayMetrics) ObserveForceUpdateBlock(platform string) {
	if platform == "" {
		platform = "unknown"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forceUpdateBlocks[platform]++
}

func (m *gatewayMetrics) ObserveRequest(group, method string, status int, duration time.Duration) {
	key := metricKey(group, method, fmt.Sprintf("%d", status))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestCount[key]++
	m.requestLatencyMS[key] += float64(duration.Milliseconds())
}

func (m *gatewayMetrics) ObserveRateLimitHit(group string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimitHits[group]++
}

func (m *gatewayMetrics) WritePrometheus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	m.mu.Lock()
	defer m.mu.Unlock()
	_, _ = fmt.Fprintln(w, "# TYPE gateway_request_count counter")
	writeMetricMap(w, "gateway_request_count", m.requestCount)
	_, _ = fmt.Fprintln(w, "# TYPE gateway_request_latency_ms_sum counter")
	writeMetricMap(w, "gateway_request_latency_ms_sum", m.requestLatencyMS)
	_, _ = fmt.Fprintln(w, "# TYPE gateway_ratelimit_hit counter")
	groups := make([]string, 0, len(m.rateLimitHits))
	for group := range m.rateLimitHits {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	for _, group := range groups {
		_, _ = fmt.Fprintf(w, "gateway_ratelimit_hit{group=%q} %d\n", group, m.rateLimitHits[group])
	}
	_, _ = fmt.Fprintln(w, "# TYPE gateway_force_update_blocks counter")
	platforms := make([]string, 0, len(m.forceUpdateBlocks))
	for platform := range m.forceUpdateBlocks {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)
	for _, platform := range platforms {
		_, _ = fmt.Fprintf(w, "gateway_force_update_blocks{platform=%q} %d\n", platform, m.forceUpdateBlocks[platform])
	}
}

func writeMetricMap[T int64 | float64](w http.ResponseWriter, name string, values map[string]T) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		if len(parts) != 3 {
			continue
		}
		_, _ = fmt.Fprintf(w, "%s{route_group=%q,method=%q,status=%q} %v\n", name, parts[0], parts[1], parts[2], values[key])
	}
}

func metricKey(parts ...string) string {
	return strings.Join(parts, "\x00")
}

func (g *gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	g.metrics.WritePrometheus(w)
}

func (g *gateway) observeRequestMetrics(r *http.Request, status int, start time.Time) {
	group := routeGroup(r)
	g.metrics.ObserveRequest(group, r.Method, status, time.Since(start))
}

func gatewayAccessLogExtras(r *http.Request) []slog.Attr {
	return []slog.Attr{
		slog.String("event", "http_access"),
		slog.String("route_group", routeGroup(r)),
		slog.String("remote_addr", r.RemoteAddr),
	}
}

func routeGroup(r *http.Request) string {
	switch {
	case r.URL.Path == "/health":
		return "health"
	case r.URL.Path == "/metrics":
		return "metrics"
	case r.URL.Path == "/ws":
		return "ws"
	case r.URL.Path == "/api/v1/version":
		return "version"
	case strings.HasPrefix(r.URL.Path, "/api/v1/"):
		namespace := restNamespace(r.URL.Path)
		if namespace != "" {
			return namespace
		}
	}
	return "unknown"
}
