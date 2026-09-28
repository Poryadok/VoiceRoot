package main

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

// T31 session routes are canonical GIS routes exposed through Gateway's
// game-integrations upstream. Their game-server bearer must reach GIS intact;
// these routes do not require a Voice user JWT.
func TestGatewayT31SessionRouteAliasesForwardRequestWithoutVoiceJWT(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create session", method: http.MethodPost, path: "/api/v1/sessions", body: `{"operation_id":"op"}`},
		{name: "close session", method: http.MethodPost, path: "/api/v1/sessions/00000000-0000-4000-8000-000000000004/close", body: `{"operation_id":"00000000-0000-4000-8000-000000000005"}`},
		{name: "operation status", method: http.MethodGet, path: "/api/v1/operations/00000000-0000-4000-8000-000000000001?include=receipts"},
		{name: "claim session event", method: http.MethodPost, path: "/api/v1/session-events/claim"},
		{name: "ack session event", method: http.MethodPost, path: "/api/v1/session-events/00000000-0000-4000-8000-000000000002/ack", body: `{"lease_id":"lease","payload_sha256":"digest"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotURI, gotAuthorization, gotBody string
			responseBody := []byte("{\"opaque\":true,\"wire\":\"bytes\"}\n")
			upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotURI, gotAuthorization = r.Method, r.URL.RequestURI(), r.Header.Get("Authorization")
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read forwarded body: %v", err)
				}
				gotBody = string(body)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", "identity")
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("X-Voice-Event-Id", "00000000-0000-4000-8000-000000000002")
				w.Header().Set("X-Voice-Payload-SHA256", "abcdef")
				w.Header().Set("X-Voice-Claim-Lease-Id", "00000000-0000-4000-8000-000000000003")
				w.Header().Set("X-Voice-Claim-Lease-Expires-At", "2026-09-28T12:00:30Z")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(responseBody)
			})
			h := newGateway(gatewayConfig{
				restUpstreams: map[string]http.Handler{"game-integrations": upstream},
			})
			rec := performRequest(h, tt.method, tt.path, tt.body, map[string]string{
				"Authorization": "Bearer vgi1_credential_secret",
				"Content-Type":  "application/json",
			})

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
			}
			if gotMethod != tt.method || gotURI != tt.path {
				t.Errorf("forwarded request = %s %q, want %s %q", gotMethod, gotURI, tt.method, tt.path)
			}
			if gotAuthorization != "Bearer vgi1_credential_secret" {
				t.Errorf("forwarded Authorization = %q, game credential must reach GIS unchanged", gotAuthorization)
			}
			if !bytes.Equal([]byte(gotBody), []byte(tt.body)) {
				t.Errorf("forwarded body = %q, want exact bytes %q", gotBody, tt.body)
			}
			if !bytes.Equal(rec.Body.Bytes(), responseBody) {
				t.Errorf("response body = %q, want exact upstream bytes %q", rec.Body.Bytes(), responseBody)
			}
			for name, want := range map[string]string{
				"Content-Type": "application/json", "Content-Encoding": "identity", "Cache-Control": "no-store",
				"X-Voice-Event-Id":       "00000000-0000-4000-8000-000000000002",
				"X-Voice-Payload-SHA256": "abcdef", "X-Voice-Claim-Lease-Id": "00000000-0000-4000-8000-000000000003",
				"X-Voice-Claim-Lease-Expires-At": "2026-09-28T12:00:30Z",
			} {
				if got := rec.Header().Get(name); got != want {
					t.Errorf("response header %s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestGatewayT31OnlyDocumentedSessionMethodsBypassVoiceJWT(t *testing.T) {
	forwarded := false
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forwarded = true
		w.WriteHeader(http.StatusNoContent)
	})
	h := newGateway(gatewayConfig{restUpstreams: map[string]http.Handler{"game-integrations": upstream}})

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/sessions"},
		{http.MethodGet, "/api/v1/sessions/00000000-0000-4000-8000-000000000004/close"},
		{http.MethodPost, "/api/v1/sessions/00000000-0000-4000-8000-000000000004/close/extra"},
		{http.MethodPost, "/api/v1/sessions/00000000-0000-4000-8000-000000000004"},
		{http.MethodPost, "/api/v1/operations/00000000-0000-4000-8000-000000000001"},
		{http.MethodDelete, "/api/v1/session-events/00000000-0000-4000-8000-000000000002/ack"},
		{http.MethodGet, "/api/v1/users/me"},
	} {
		forwarded = false
		rec := performRequest(h, tc.method, tc.path, "", map[string]string{"Authorization": "Bearer vgi1_game_server"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want Voice-JWT 401", tc.method, tc.path, rec.Code)
		}
		if forwarded {
			t.Errorf("%s %s reached an upstream without a Voice JWT", tc.method, tc.path)
		}
	}
}
