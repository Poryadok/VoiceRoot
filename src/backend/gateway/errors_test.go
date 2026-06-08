package main

import (
	"net/http"
	"testing"
)

// REST proxy passes upstream status and body unchanged when the transcoder does not handle the route.
func TestRESTProxyPassthroughPreservesDownstreamErrorBody(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		restUpstreams: map[string]http.Handler{
			"bots": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error_code":"permission_denied","message":"not allowed"}`))
			}),
		},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/bots/hooks", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	var got struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	decodeJSON(t, rec.Body, &got)
	if got.ErrorCode != "permission_denied" || got.Message != "not allowed" {
		t.Fatalf("error body = %+v", got)
	}
}
