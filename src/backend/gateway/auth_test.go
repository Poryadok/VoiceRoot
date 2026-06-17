package main

import (
	"net/http"
	"testing"
)

func TestAuthBoundary(t *testing.T) {
	t.Parallel()

	var downstream http.Header
	h := newGatewayForContract(t, gatewayTestOptions{
		versionConfigs: map[string]versionConfig{
			"android": {
				MinSupportedVersion: "1.4.0",
				LatestVersion:       "1.7.2",
			},
		},
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {
				UserID:           "account-1",
				ProfileID:        "profile-1",
				Roles:            []string{"member"},
				SubscriptionTier: "free",
			},
			"staff-token": {
				UserID:           "staff-account",
				ProfileID:        "staff-profile",
				Roles:            []string{"staff"},
				SubscriptionTier: "premium",
			},
		},
		restUpstreams: map[string]http.Handler{
			"auth": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
			"users": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				downstream = r.Header.Clone()
				w.WriteHeader(http.StatusNoContent)
			}),
			"analytics": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	publicRoutes := []struct {
		method string
		route  string
		body   string
	}{
		{method: http.MethodPost, route: "/api/v1/auth/login", body: `{}`},
		{method: http.MethodPost, route: "/api/v1/auth/register", body: `{}`},
		{method: http.MethodPost, route: "/api/v1/auth/refresh", body: `{}`},
		{method: http.MethodGet, route: "/api/v1/auth/oauth2/authorize?response_type=code&client_id=test&redirect_uri=http://localhost/cb&state=s&code_challenge=x&code_challenge_method=S256"},
		{method: http.MethodPost, route: "/api/v1/auth/oauth2/authorize", body: "response_type=code&client_id=test&redirect_uri=http://localhost/cb&email=u%40example.com&password=secret"},
		{method: http.MethodPost, route: "/api/v1/auth/oauth2/token", body: "grant_type=authorization_code"},
		{method: http.MethodGet, route: "/api/v1/auth/.well-known/openid-configuration"},
		{method: http.MethodGet, route: "/api/v1/version?platform=android&version=1.7.2"},
	}
	for _, publicRoute := range publicRoutes {
		publicRoute := publicRoute
		t.Run("public "+publicRoute.route, func(t *testing.T) {
			rec := performRequest(h, publicRoute.method, publicRoute.route, publicRoute.body, nil)
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("%s unexpectedly required JWT", publicRoute.route)
			}
			if rec.Code == http.StatusNotFound {
				t.Fatalf("%s was not registered as a public Gateway route", publicRoute.route)
			}
		})
	}

	protected := performRequest(h, http.MethodGet, "/api/v1/users/me", "", nil)
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("protected status = %d, want %d; body=%q", protected.Code, http.StatusUnauthorized, protected.Body.String())
	}

	authorized := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d, want %d; body=%q", authorized.Code, http.StatusNoContent, authorized.Body.String())
	}
	for header, want := range map[string]string{
		"X-Voice-User-Id":           "account-1",
		"X-Voice-Profile-Id":        "profile-1",
		"X-Voice-Roles":             "member",
		"X-Voice-Subscription-Tier": "free",
	} {
		if got := downstream.Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}

	notStaff := performRequest(h, http.MethodGet, "/api/v1/analytics/reports", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if notStaff.Code != http.StatusForbidden {
		t.Fatalf("analytics non-staff status = %d, want %d", notStaff.Code, http.StatusForbidden)
	}

	staff := performRequest(h, http.MethodGet, "/api/v1/analytics/reports", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	if staff.Code != http.StatusNoContent {
		t.Fatalf("analytics staff status = %d, want %d; body=%q", staff.Code, http.StatusNoContent, staff.Body.String())
	}
}
