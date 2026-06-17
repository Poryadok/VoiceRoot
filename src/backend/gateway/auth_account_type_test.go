package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAuthBoundary_ForwardsAccountTypeHeader documents Gateway → downstream X-Voice-Account-Type
// from JWT account_type claim (docs/TODO.md guest accounts).
func TestAuthBoundary_ForwardsAccountTypeHeader(t *testing.T) {
	t.Parallel()

	var downstream http.Header
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"guest-dev-token": {
				UserID:           "guest-account",
				ProfileID:        "guest-profile",
				Roles:            []string{"user"},
				SubscriptionTier: "free",
				AccountType:      "guest",
			},
			"regular-dev-token": {
				UserID:           "regular-account",
				ProfileID:        "regular-profile",
				Roles:            []string{"user"},
				SubscriptionTier: "free",
				AccountType:      "regular",
			},
		},
		restUpstreams: map[string]http.Handler{
			"users": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				downstream = r.Header.Clone()
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	guest := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization": "Bearer guest-dev-token",
	})
	require.Equal(t, http.StatusNoContent, guest.Code)
	require.Equal(t, "guest", downstream.Get("X-Voice-Account-Type"))

	regular := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization": "Bearer regular-dev-token",
	})
	require.Equal(t, http.StatusNoContent, regular.Code)
	require.Equal(t, "regular", downstream.Get("X-Voice-Account-Type"))
}
