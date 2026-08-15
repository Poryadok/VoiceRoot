package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	voiceprom "voice/backend/pkg/promhttp"
)

func TestHealthHandler_MetricsEndpoint(t *testing.T) {
	reg := prometheus.NewRegistry()
	h := voiceprom.MountMetricsOnHealth(healthHandler(serviceName), reg)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain"))
}
