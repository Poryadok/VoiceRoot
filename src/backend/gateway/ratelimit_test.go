package main

import (
	"net/http"
	"testing"
)

func TestRateLimitGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		target     string
		body       string
		group      string
		authHeader string
	}{
		{name: "auth login", method: http.MethodPost, target: "/api/v1/auth/login", body: `{}`, group: "Auth"},
		{name: "auth register", method: http.MethodPost, target: "/api/v1/auth/register", body: `{}`, group: "Auth"},
		{name: "otp", method: http.MethodPost, target: "/api/v1/auth/otp/send", body: `{}`, group: "OTP"},
		{name: "messages send", method: http.MethodPost, target: "/api/v1/messages/send", body: `{"text":"hi"}`, group: "MessagesSend", authHeader: "Bearer valid-user-token"},
		{name: "file upload", method: http.MethodPost, target: "/api/v1/files/upload", body: `file`, group: "FileUpload", authHeader: "Bearer valid-user-token"},
		{name: "avatar presigned upload", method: http.MethodPost, target: "/api/v1/users/me/avatar/presigned-upload", body: `{"content_type":"image/png","content_length":1024}`, group: "FileUpload", authHeader: "Bearer valid-user-token"},
		{name: "space creation", method: http.MethodPost, target: "/api/v1/spaces", body: `{}`, group: "SpaceCreation", authHeader: "Bearer valid-user-token"},
		{name: "bot api", method: http.MethodPost, target: "/api/v1/bots/interactions", body: `{}`, group: "BotAPI", authHeader: "Bearer valid-user-token"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name+" limited", func(t *testing.T) {
			t.Parallel()

			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims: map[string]tokenClaims{
					"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
				},
				rateLimitedGroups: map[string]bool{tc.group: true},
				restUpstreams: map[string]http.Handler{
					"auth":     http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"files":    http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"users":    http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"spaces":   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"bots":     http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
				},
			})
			headers := map[string]string{"X-Forwarded-For": "203.0.113.10"}
			if tc.authHeader != "" {
				headers["Authorization"] = tc.authHeader
			}

			rec := performRequest(h, tc.method, tc.target, tc.body, headers)
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("status = %d, want %d for group %s; body=%q", rec.Code, http.StatusTooManyRequests, tc.group, rec.Body.String())
			}
		})
	}
}
