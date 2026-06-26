package main

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	grpcmw "voice/backend/pkg/grpcmw"
)

type gatewayMetrics struct {
	registry          *prometheus.Registry
	httpRequests      *prometheus.CounterVec
	httpDuration      *prometheus.HistogramVec
	rateLimitHits     *prometheus.CounterVec
	forceUpdateBlocks *prometheus.CounterVec
	wsProxyActive     prometheus.Gauge
}

func newGatewayMetrics() *gatewayMetrics {
	reg := prometheus.NewRegistry()
	m := &gatewayMetrics{
		registry: reg,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_http_requests_total",
			Help: "Total HTTP requests handled by the gateway.",
		}, []string{"route_group", "method", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: grpcmw.DefaultHistogramBuckets,
		}, []string{"route_group", "method"}),
		rateLimitHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_ratelimit_hits_total",
			Help: "Total requests blocked by gateway rate limits.",
		}, []string{"group"}),
		forceUpdateBlocks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_force_update_blocks_total",
			Help: "Total requests blocked by force-update policy.",
		}, []string{"platform"}),
		wsProxyActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_ws_proxy_active",
			Help: "Active WebSocket connections proxied to realtime upstream.",
		}),
	}
	reg.MustRegister(
		m.httpRequests,
		m.httpDuration,
		m.rateLimitHits,
		m.forceUpdateBlocks,
		m.wsProxyActive,
	)
	return m
}

func (m *gatewayMetrics) ObserveForceUpdateBlock(platform string) {
	if platform == "" {
		platform = "unknown"
	}
	m.forceUpdateBlocks.WithLabelValues(platform).Inc()
}

func (m *gatewayMetrics) ObserveRequest(group, method string, status int, duration time.Duration) {
	statusStr := strconv.Itoa(status)
	m.httpRequests.WithLabelValues(group, method, statusStr).Inc()
	m.httpDuration.WithLabelValues(group, method).Observe(duration.Seconds())
}

func (m *gatewayMetrics) ObserveRateLimitHit(group string) {
	m.rateLimitHits.WithLabelValues(group).Inc()
}

func (m *gatewayMetrics) wrapRealtimeProxy(upstream http.Handler) http.Handler {
	if upstream == nil {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.ServeHTTP(&wsProxyResponseWriter{
			ResponseWriter: w,
			onHijack:       m.trackWSProxyConn,
		}, r)
	})
}

func (m *gatewayMetrics) trackWSProxyConn(conn net.Conn) net.Conn {
	m.wsProxyActive.Inc()
	return &closeNotifyConn{
		Conn: conn,
		onClose: func() {
			m.wsProxyActive.Dec()
		},
	}
}

type wsProxyResponseWriter struct {
	http.ResponseWriter
	onHijack func(net.Conn) net.Conn
}

func (w *wsProxyResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	conn, rw, err := h.Hijack()
	if err != nil {
		return conn, rw, err
	}
	if w.onHijack != nil {
		conn = w.onHijack(conn)
	}
	return conn, rw, nil
}

type closeNotifyConn struct {
	net.Conn
	onClose func()
	once    sync.Once
}

func (c *closeNotifyConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.onClose)
	return err
}

func (g *gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	promhttp.HandlerFor(g.metrics.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
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
