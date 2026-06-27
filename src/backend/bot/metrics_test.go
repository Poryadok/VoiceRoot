package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/grpcmw"
	voiceprom "voice/backend/pkg/promhttp"
)

func TestHealthHandler_MetricsExposesGRPCServer(t *testing.T) {
	reg := prometheus.NewRegistry()
	_ = grpcmw.UnaryMetricsForRegistry(reg)

	h := voiceprom.MountMetricsOnHealth(healthHandler(serviceName), reg)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	text := string(body)
	require.Contains(t, text, "grpc_server_handled_total")
	require.Contains(t, text, "grpc_server_handling_seconds")
	require.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain"))
}
