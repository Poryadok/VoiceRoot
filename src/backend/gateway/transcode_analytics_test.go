package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyticsStaffGateAndTranscode(t *testing.T) {
	clients := grpcClientsFromEnv(nil)
	if clients == nil {
		clients = &grpcClients{}
	}
	// Force nil analytics to test staff gate via transcoder false -> 404; staff gate runs before transcoder.
	h := newGateway(gatewayConfig{
		tokenClaims: map[string]tokenClaims{
			"staff": {UserID: "u1", ProfileID: "p1", Roles: []string{"staff"}},
			"user":  {UserID: "u2", ProfileID: "p2"},
		},
		transcoder: newTranscoder(clients),
	})

	notStaff := performRequest(h, http.MethodGet, "/api/v1/analytics/dashboard/product", "", map[string]string{
		"Authorization": "Bearer user",
	})
	require.Equal(t, http.StatusForbidden, notStaff.Code)

	noUpstream := performRequest(h, http.MethodGet, "/api/v1/analytics/dashboard/product", "", map[string]string{
		"Authorization": "Bearer staff",
	})
	require.True(t, noUpstream.Code == http.StatusNotFound || noUpstream.Code == http.StatusServiceUnavailable)
}
