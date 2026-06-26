package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpointExposesPrometheusMetrics(t *testing.T) {
	t.Parallel()

	metrics := newGatewayMetrics()
	h := newGatewayForContract(t, gatewayTestOptions{
		metrics:           metrics,
		rateLimitedGroups: map[string]bool{"AuthLogin": true},
		restUpstreams: map[string]http.Handler{
			"auth": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	_ = performRequest(h, http.MethodPost, "/api/v1/auth/login", "{}", nil)
	_ = performRequest(h, http.MethodPost, "/api/v1/auth/login", "{}", nil)

	rec := performRequest(h, http.MethodGet, "/metrics", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("content-type = %q, want Prometheus text format", ct)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"# TYPE gateway_http_requests_total counter",
		"gateway_http_requests_total{",
		"# TYPE gateway_http_request_duration_seconds histogram",
		"gateway_http_request_duration_seconds_bucket{",
		"gateway_http_request_duration_seconds_sum",
		"gateway_http_request_duration_seconds_count",
		"# TYPE gateway_ratelimit_hits_total counter",
		"gateway_ratelimit_hits_total",
		"# TYPE gateway_ws_proxy_active gauge",
		"gateway_ws_proxy_active",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}

	legacy := []string{
		"gateway_request_count",
		"gateway_request_latency_ms_sum",
		"gateway_ratelimit_hit{",
		"gateway_force_update_blocks{",
	}
	for _, name := range legacy {
		if strings.Contains(body, name) {
			t.Fatalf("metrics body still contains legacy metric %q:\n%s", name, body)
		}
	}
}

func TestMetricsHistogramBucketsMatchSpec(t *testing.T) {
	t.Parallel()

	metrics := newGatewayMetrics()
	h := newGatewayForContract(t, gatewayTestOptions{
		metrics: metrics,
		restUpstreams: map[string]http.Handler{
			"health": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		},
	})

	_ = performRequest(h, http.MethodGet, "/health", "", nil)
	rec := performRequest(h, http.MethodGet, "/metrics", "", nil)
	body := rec.Body.String()

	for _, le := range []string{"0.005", "0.01", "0.025", "0.05", "0.1", "0.25", "0.5", "1", "2.5", "5", "10"} {
		needle := `gateway_http_request_duration_seconds_bucket{method="GET",route_group="health",le="` + le + `"}`
		if !strings.Contains(body, needle) {
			t.Fatalf("metrics body missing bucket %s:\n%s", le, body)
		}
	}
}

func TestMetricsRateLimitHitCounter(t *testing.T) {
	t.Parallel()

	metrics := newGatewayMetrics()
	h := newGatewayForContract(t, gatewayTestOptions{
		metrics:           metrics,
		rateLimitedGroups: map[string]bool{"AuthLogin": true},
		restUpstreams: map[string]http.Handler{
			"auth": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	limited := performRequest(h, http.MethodPost, "/api/v1/auth/login", "{}", nil)
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d", limited.Code)
	}

	rec := performRequest(h, http.MethodGet, "/metrics", "", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `gateway_ratelimit_hits_total{group="AuthLogin"} 1`) {
		t.Fatalf("metrics body = %q", body)
	}
}

func TestWSProxyActiveGaugeOnHijack(t *testing.T) {
	t.Parallel()

	metrics := newGatewayMetrics()
	var proxiedConn net.Conn
	h := newGatewayForContract(t, gatewayTestOptions{
		metrics: metrics,
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "no hijack", http.StatusInternalServerError)
				return
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			proxiedConn = conn
			_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n"))
		}),
	})

	server := httptest.NewServer(h)
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer valid-user-token")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 101; body=%q", resp.StatusCode, body)
	}

	metricsRec := performRequest(h, http.MethodGet, "/metrics", "", nil)
	if !strings.Contains(metricsRec.Body.String(), "gateway_ws_proxy_active 1") {
		t.Fatalf("expected active WS gauge = 1:\n%s", metricsRec.Body.String())
	}

	if proxiedConn != nil {
		_ = proxiedConn.Close()
	}

	metricsRec = performRequest(h, http.MethodGet, "/metrics", "", nil)
	if strings.Contains(metricsRec.Body.String(), "gateway_ws_proxy_active 1") {
		t.Fatalf("expected gauge to drop after close:\n%s", metricsRec.Body.String())
	}
}

func TestWSProxyActiveGaugeNoHijackStaysZero(t *testing.T) {
	t.Parallel()

	metrics := newGatewayMetrics()
	h := newGatewayForContract(t, gatewayTestOptions{
		metrics: metrics,
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusSwitchingProtocols)
		}),
	})

	rec := performRequest(h, http.MethodGet, "/ws", "", map[string]string{
		"Authorization":         "Bearer valid-user-token",
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	})
	if rec.Code != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101", rec.Code)
	}

	metricsRec := performRequest(h, http.MethodGet, "/metrics", "", nil)
	if !strings.Contains(metricsRec.Body.String(), "gateway_ws_proxy_active 0") {
		t.Fatalf("expected gauge 0 without hijack:\n%s", metricsRec.Body.String())
	}
}

// Ensure closeNotifyConn decrements gauge (unit-level).
func TestCloseNotifyConnCallsOnCloseOnce(t *testing.T) {
	t.Parallel()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	calls := 0
	tracked := &closeNotifyConn{
		Conn: c1,
		onClose: func() {
			calls++
		},
	}
	_ = tracked.Close()
	_ = tracked.Close()
	if calls != 1 {
		t.Fatalf("onClose calls = %d, want 1", calls)
	}
}
