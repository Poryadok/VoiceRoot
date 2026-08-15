package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyticsExportWritesAuditStore(t *testing.T) {
	t.Parallel()
	store := newMemoryAnalyticsAuditStore(10)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"staff-token": {UserID: "staff-1", ProfileID: "staff-profile", Roles: []string{"staff"}},
		},
		analyticsAudit: store,
		restUpstreams: map[string]http.Handler{
			"analytics": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/csv")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event_type,count\n"))
			}),
		},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/analytics/export?format=csv", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	entries, err := store.Recent(t.Context(), 5)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	require.Equal(t, "/api/v1/analytics/export", entries[len(entries)-1].Route)
	require.Equal(t, "staff-1", entries[len(entries)-1].UserID)
	require.Equal(t, "staff-profile", entries[len(entries)-1].ProfileID)
}
