package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type gatewayTestOptions struct {
	versionConfigs     map[string]versionConfig
	forceUpdate        *forceUpdatePolicy
	tokenClaims        map[string]tokenClaims
	rateLimitedGroups  map[string]bool
	restUpstreams      map[string]http.Handler
	realtimeUpstream   http.Handler
	requestIDGenerator func() string
}

type versionConfig struct {
	MinSupportedVersion string
	LatestVersion       string
	UpdateURL           string
	ReleaseNotes        string
	ShorebirdPatch      int
}

type forceUpdatePolicy struct {
	Platform  string
	Version   string
	UpdateURL string
}

type tokenClaims struct {
	UserID           string
	ProfileID        string
	Roles            []string
	SubscriptionTier string
}

func newGatewayForContract(_ *testing.T, _ gatewayTestOptions) http.Handler {
	return handler()
}

func performRequest(h http.Handler, method, target string, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, body io.Reader, dst any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   string
	}{
		{name: "GET returns ok", method: http.MethodGet, wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "POST is not accepted as a health check", method: http.MethodPost, wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := performRequest(handler(), tc.method, "/health", "", nil)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
			if tc.wantStatus == http.StatusOK {
				if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
					t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
				}
			}
		})
	}
}

func TestVersionEndpoint(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		versionConfigs: map[string]versionConfig{
			"android": {
				MinSupportedVersion: "1.4.0",
				LatestVersion:       "1.7.2",
				UpdateURL:           "https://updates.voice.example/android",
				ReleaseNotes:        "Android voice fixes",
				ShorebirdPatch:      42,
			},
		},
	})

	tests := []struct {
		name                string
		target              string
		wantForceUpdate     bool
		wantUpdateAvailable bool
	}{
		{
			name:                "current supported version reports no update",
			target:              "/api/v1/version?platform=android&version=1.7.2&shorebird_patch=42",
			wantForceUpdate:     false,
			wantUpdateAvailable: false,
		},
		{
			name:                "below minimum version forces update",
			target:              "/api/v1/version?platform=android&version=1.3.9&shorebird_patch=7",
			wantForceUpdate:     true,
			wantUpdateAvailable: true,
		},
		{
			name:                "below latest but still supported offers soft update",
			target:              "/api/v1/version?platform=android&version=1.6.0&shorebird_patch=12",
			wantForceUpdate:     false,
			wantUpdateAvailable: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := performRequest(h, http.MethodGet, tc.target, "", nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}

			var got struct {
				ForceUpdate        bool   `json:"force_update"`
				UpdateAvailable    bool   `json:"update_available"`
				LatestVersion      string `json:"latest_version"`
				MinSupported       string `json:"min_supported_version"`
				UpdateURL          string `json:"update_url"`
				ReleaseNotes       string `json:"release_notes"`
				ShorebirdPatch     int    `json:"shorebird_patch"`
				UnexpectedJSONKeys map[string]json.RawMessage
			}
			decodeJSON(t, rec.Body, &got)

			if got.ForceUpdate != tc.wantForceUpdate {
				t.Fatalf("force_update = %v, want %v", got.ForceUpdate, tc.wantForceUpdate)
			}
			if got.UpdateAvailable != tc.wantUpdateAvailable {
				t.Fatalf("update_available = %v, want %v", got.UpdateAvailable, tc.wantUpdateAvailable)
			}
			if got.LatestVersion != "1.7.2" || got.MinSupported != "1.4.0" {
				t.Fatalf("versions = latest %q min %q, want latest 1.7.2 min 1.4.0", got.LatestVersion, got.MinSupported)
			}
			if got.UpdateURL != "https://updates.voice.example/android" {
				t.Fatalf("update_url = %q", got.UpdateURL)
			}
			if got.ReleaseNotes != "Android voice fixes" {
				t.Fatalf("release_notes = %q", got.ReleaseNotes)
			}
			if got.ShorebirdPatch != 42 {
				t.Fatalf("shorebird_patch = %d, want 42", got.ShorebirdPatch)
			}
		})
	}
}

func TestForceUpdateBlocksEveryAPIRouteExceptVersion(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		forceUpdate: &forceUpdatePolicy{
			Platform:  "android",
			Version:   "1.3.9",
			UpdateURL: "https://updates.voice.example/android",
		},
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	blocked := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{"text":"hi"}`, map[string]string{
		"Authorization":           "Bearer valid-user-token",
		"X-Voice-Client-Platform": "android",
		"X-Voice-Client-Version":  "1.3.9",
	})
	if blocked.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d; body=%q", blocked.Code, http.StatusUpgradeRequired, blocked.Body.String())
	}
	var got struct {
		Error     string `json:"error"`
		UpdateURL string `json:"update_url"`
	}
	decodeJSON(t, blocked.Body, &got)
	if got.Error != "client_outdated" || got.UpdateURL != "https://updates.voice.example/android" {
		t.Fatalf("error body = %+v", got)
	}

	version := performRequest(h, http.MethodGet, "/api/v1/version?platform=android&version=1.3.9", "", nil)
	if version.Code == http.StatusUpgradeRequired {
		t.Fatalf("/api/v1/version must not be blocked by force-update policy")
	}
}

func TestAuthBoundary(t *testing.T) {
	t.Parallel()

	var downstream http.Header
	h := newGatewayForContract(t, gatewayTestOptions{
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

	publicRoutes := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/version?platform=android&version=1.7.2",
	}
	for _, route := range publicRoutes {
		route := route
		t.Run("public "+route, func(t *testing.T) {
			rec := performRequest(h, http.MethodPost, route, `{}`, nil)
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("%s unexpectedly required JWT", route)
			}
			if rec.Code == http.StatusNotFound {
				t.Fatalf("%s was not registered as a public Gateway route", route)
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

func TestRESTNamespaceRouting(t *testing.T) {
	t.Parallel()

	namespaces := []string{
		"auth",
		"users",
		"friends",
		"chats",
		"messages",
		"spaces",
		"roles",
		"voice",
		"files",
		"notifications",
		"search",
		"matchmaking",
		"moderation",
		"subscription",
		"bots",
		"stories",
		"analytics",
	}

	upstreams := make(map[string]http.Handler, len(namespaces))
	for _, namespace := range namespaces {
		namespace := namespace
		upstreams[namespace] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("X-Upstream-Namespace", namespace)
			w.Header().Set("X-Upstream-Path", r.URL.Path)
			w.Header().Set("X-Upstream-Query", r.URL.RawQuery)
			w.Header().Set("X-Upstream-Method", r.Method)
			w.Header().Set("X-Upstream-Body", string(body))
			w.WriteHeader(http.StatusAccepted)
		})
	}

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"staff-token": {UserID: "staff-account", Roles: []string{"staff"}},
		},
		restUpstreams: upstreams,
	})

	for _, namespace := range namespaces {
		namespace := namespace
		t.Run(namespace, func(t *testing.T) {
			t.Parallel()

			rec := performRequest(h, http.MethodPatch, "/api/v1/"+namespace+"/resource/42?cursor=abc", "payload", map[string]string{
				"Authorization": "Bearer staff-token",
			})
			if rec.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusAccepted, rec.Body.String())
			}
			if got := rec.Header().Get("X-Upstream-Namespace"); got != namespace {
				t.Fatalf("namespace = %q, want %q", got, namespace)
			}
			if got := rec.Header().Get("X-Upstream-Path"); got != "/api/v1/"+namespace+"/resource/42" {
				t.Fatalf("path = %q", got)
			}
			if got := rec.Header().Get("X-Upstream-Query"); got != "cursor=abc" {
				t.Fatalf("query = %q", got)
			}
			if got := rec.Header().Get("X-Upstream-Method"); got != http.MethodPatch {
				t.Fatalf("method = %q", got)
			}
			if got := rec.Header().Get("X-Upstream-Body"); got != "payload" {
				t.Fatalf("body = %q", got)
			}
		})
	}

	unknown := performRequest(h, http.MethodGet, "/api/v1/unknown/resource", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown namespace status = %d, want %d", unknown.Code, http.StatusNotFound)
	}

	federation := performRequest(h, http.MethodGet, "/api/v1/federation/nodes", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	if federation.Code != http.StatusNotFound {
		t.Fatalf("federation must stay outside public Gateway REST; status = %d", federation.Code)
	}
}

func TestWebSocketBoundary(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Upstream-Namespace", "realtime")
			w.WriteHeader(http.StatusSwitchingProtocols)
		}),
	})

	plain := performRequest(h, http.MethodGet, "/ws", "", nil)
	if plain.Code != http.StatusBadRequest {
		t.Fatalf("plain /ws status = %d, want %d", plain.Code, http.StatusBadRequest)
	}

	upgradeWithoutJWT := performRequest(h, http.MethodGet, "/ws", "", map[string]string{
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	})
	if upgradeWithoutJWT.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /ws upgrade status = %d, want %d", upgradeWithoutJWT.Code, http.StatusUnauthorized)
	}

	upgrade := performRequest(h, http.MethodGet, "/ws", "", map[string]string{
		"Authorization":         "Bearer valid-user-token",
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	})
	if upgrade.Code != http.StatusSwitchingProtocols {
		t.Fatalf("authenticated /ws upgrade status = %d, want %d; body=%q", upgrade.Code, http.StatusSwitchingProtocols, upgrade.Body.String())
	}
	if got := upgrade.Header().Get("X-Upstream-Namespace"); got != "realtime" {
		t.Fatalf("/ws upstream = %q, want realtime", got)
	}
}

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
