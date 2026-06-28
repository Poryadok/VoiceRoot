package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	voiceprom "voice/backend/pkg/promhttp"
)

func testRealtimeHandlerWithMetrics(tv tokenValidator, lister dmChatLister, reg *prometheus.Registry) http.Handler {
	initRealtimeMetrics(reg)
	base := newServiceHandler(serviceName, tv, lister, newWSHub(), nil, "test-instance")
	return voiceprom.MountMetricsOnHealth(base, reg)
}

func TestMetricsEndpointExposesRealtimeWSMetrics(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := testRealtimeHandlerWithMetrics(staticTokenValidator{"tok": {UserID: "a", ProfileID: "p"}}, nil, reg)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	text := string(body)
	for _, want := range []string{
		"# TYPE realtime_ws_connections_active gauge",
		"realtime_ws_connections_active",
		"# TYPE realtime_ws_connect_total counter",
		"realtime_ws_connect_total",
		"# TYPE realtime_ws_hello_duration_seconds histogram",
		"realtime_ws_hello_duration_seconds_bucket",
		"realtime_ws_hello_duration_seconds_sum",
		"realtime_ws_hello_duration_seconds_count",
	} {
		require.Contains(t, text, want)
	}
	require.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain"))
}

func TestWSMetricsConnectSuccessAndHelloDuration(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := testRealtimeHandlerWithMetrics(staticTokenValidator{"tok": {UserID: "a", ProfileID: "p"}}, nil, reg)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "p")
	conn, _, err := websocket.DefaultDialer.Dial(u, hdr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	var frame wsOutbound
	require.NoError(t, conn.ReadJSON(&frame))
	require.Equal(t, "hello", frame.Op)

	require.Equal(t, float64(1), testutil.ToFloat64(rtMetrics.connectTotal.WithLabelValues("success")))
	require.Equal(t, float64(0), testutil.ToFloat64(rtMetrics.connectTotal.WithLabelValues("fail")))
	require.Equal(t, float64(1), testutil.ToFloat64(rtMetrics.connectionsActive))
	require.Equal(t, 1, testutil.CollectAndCount(rtMetrics.helloDuration))
}

func TestWSMetricsConnectFail(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := testRealtimeHandlerWithMetrics(staticTokenValidator{"tok": {UserID: "a", ProfileID: "p"}}, nil, reg)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	_, resp, err := websocket.DefaultDialer.Dial(u, wsUpgradeHeaders(""))
	require.Error(t, err)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	require.Equal(t, float64(1), testutil.ToFloat64(rtMetrics.connectTotal.WithLabelValues("fail")))
	require.Equal(t, float64(0), testutil.ToFloat64(rtMetrics.connectTotal.WithLabelValues("success")))
}

func TestWSMetricsActiveConnectionsDecrementOnClose(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := testRealtimeHandlerWithMetrics(staticTokenValidator{"tok": {UserID: "a", ProfileID: "p"}}, nil, reg)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "p")
	conn, _, err := websocket.DefaultDialer.Dial(u, hdr)
	require.NoError(t, err)

	var frame wsOutbound
	require.NoError(t, conn.ReadJSON(&frame))
	require.Equal(t, "hello", frame.Op)
	require.Equal(t, float64(1), testutil.ToFloat64(rtMetrics.connectionsActive))

	require.NoError(t, conn.Close())
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(rtMetrics.connectionsActive) == 0
	}, 2*time.Second, 20*time.Millisecond)
}
