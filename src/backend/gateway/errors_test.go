package main

import (
	"net/http"
	"testing"
)

func TestDownstreamErrorMapping(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Voice-GRPC-Code", "PERMISSION_DENIED")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error_code":"permission_denied","message":"not allowed"}`))
			}),
		},
	})

	rec := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{}`, map[string]string{
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
	if got.ErrorCode != "permission_denied" || got.Message == "" {
		t.Fatalf("error body = %+v", got)
	}
}
