package main

import (
	"net/http"
	"testing"
)

func TestRequestIDGenerationAndPropagation(t *testing.T) {
	t.Parallel()

	var downstream http.Header
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		requestIDGenerator: func() string { return "generated-request-id" },
		restUpstreams: map[string]http.Handler{
			"users": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				downstream = r.Header.Clone()
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	generated := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if generated.Code != http.StatusNoContent {
		t.Fatalf("generated request id status = %d, want %d", generated.Code, http.StatusNoContent)
	}
	if got := generated.Header().Get("X-Request-Id"); got != "generated-request-id" {
		t.Fatalf("response X-Request-Id = %q, want generated-request-id", got)
	}
	if got := downstream.Get("X-Request-Id"); got != "generated-request-id" {
		t.Fatalf("downstream X-Request-Id = %q, want generated-request-id", got)
	}

	preserved := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
		"X-Request-Id":  "client-request-id",
	})
	if preserved.Code != http.StatusNoContent {
		t.Fatalf("preserved request id status = %d, want %d", preserved.Code, http.StatusNoContent)
	}
	if got := preserved.Header().Get("X-Request-Id"); got != "client-request-id" {
		t.Fatalf("response X-Request-Id = %q, want client-request-id", got)
	}
	if got := downstream.Get("X-Request-Id"); got != "client-request-id" {
		t.Fatalf("downstream X-Request-Id = %q, want client-request-id", got)
	}
}
