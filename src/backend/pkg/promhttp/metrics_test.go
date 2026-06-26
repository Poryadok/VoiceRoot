package promhttp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	voiceprom "voice/backend/pkg/promhttp"
)

func TestRegister_ExposesMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewCounter(prometheus.CounterOpts{Name: "test_metric_total", Help: "test"}))

	mux := http.NewServeMux()
	voiceprom.Register(mux, reg)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "test_metric_total")
	require.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain"))
}

func TestMountMetricsOnHealth_SameMux(t *testing.T) {
	reg := prometheus.NewRegistry()
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := voiceprom.MountMetricsOnHealth(healthMux, reg)
	require.Same(t, healthMux, h)

	rec := httptest.NewRecorder()
	mux := h.(*http.ServeMux)
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}
