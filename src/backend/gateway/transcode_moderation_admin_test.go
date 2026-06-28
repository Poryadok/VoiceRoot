package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTranscodeModerationAdmin_staffRequired documents Phase 14 moderator admin REST is staff-only.
func TestTranscodeModerationAdmin_staffRequired(t *testing.T) {
	t.Parallel()

	rec := &recordingModerationAdmin{}
	h := newModerationAdminContractGateway(t, rec)

	staffRoutes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/admin/moderation/reports?status=pending&queue=content", ""},
		{http.MethodGet, "/api/v1/admin/moderation/reports?status=pending&queue=spaces", ""},
		{http.MethodPost, "/api/v1/admin/moderation/sanctions", `{"target_account_id":"acct-1","type":"warning","reason":"test"}`},
		{http.MethodPost, "/api/v1/admin/moderation/reports/report-1/resolve", `{"new_status":"resolved","resolution_json":"{}"}`},
	}

	for _, route := range staffRoutes {
		route := route
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()
			staff := performRequest(h, route.method, route.path, route.body, map[string]string{
				"Authorization": "Bearer staff-token",
				"Content-Type":  "application/json",
			})
			require.NotEqual(t, http.StatusForbidden, staff.Code, "staff must access admin moderation routes")
			require.NotEqual(t, http.StatusUnauthorized, staff.Code, "staff token must be accepted")

			nonStaff := performRequest(h, route.method, route.path, route.body, map[string]string{
				"Authorization": "Bearer member-token",
				"Content-Type":  "application/json",
			})
			require.Equal(t, http.StatusForbidden, nonStaff.Code, "non-staff must be forbidden on admin moderation routes")
		})
	}
}

type recordingModerationAdmin struct{}

func newModerationAdminContractGateway(t *testing.T, rec *recordingModerationAdmin) http.Handler {
	t.Helper()
	modClient, cleanup := startBufconnModerationClient(t, &recordingModerationReports{})
	t.Cleanup(cleanup)
	_ = rec
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"staff-token":  {UserID: "staff-account", ProfileID: "staff-profile", Roles: []string{"staff"}},
			"member-token": {UserID: "account-1", ProfileID: "profile-1", Roles: []string{"member"}},
		},
		transcoder: &transcoder{clients: grpcClients{moderation: modClient}},
		restUpstreams: map[string]http.Handler{
			"moderation": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
			}),
		},
	})
}
