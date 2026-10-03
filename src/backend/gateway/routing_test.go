package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

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
		"game-integrations",
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

func TestGameSessionRosterRouteUsesGISCredentialAndPreservesJWTBoundaries(t *testing.T) {
	var authorization string
	var forwardedPath string
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		authorization = r.Header.Get("Authorization")
		forwardedPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	})
	h := newGatewayForContract(t, gatewayTestOptions{
		restUpstreams: map[string]http.Handler{"game-integrations": upstream},
	})

	const credential = "vgi1.eyJhcHBsaWNhdGlvbl9pZCI6ImFwcCIsImVudmlyb25tZW50X2lkIjoiZW52In0.signature"
	path := "/api/v1/sessions/11111111-1111-4111-8111-111111111111/roster"
	rec := performRequest(h, http.MethodPut, path, `{"roster_complete":true}`, map[string]string{
		"Authorization": "Bearer " + credential,
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("roster status = %d, want %d; body=%q", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if authorization != "Bearer "+credential {
		t.Fatalf("GIS authorization = %q, want original vgi1 bearer", authorization)
	}
	if forwardedPath != path {
		t.Fatalf("GIS path = %q, want %q", forwardedPath, path)
	}

	// Only the exact route authenticated by GIS may bypass Voice JWT validation.
	neighbor := performRequest(h, http.MethodGet, "/api/v1/sessions/11111111-1111-4111-8111-111111111111", "", nil)
	if neighbor.Code != http.StatusUnauthorized {
		t.Fatalf("neighboring session route status = %d, want %d", neighbor.Code, http.StatusUnauthorized)
	}
	if upstreamCalls != 1 {
		t.Fatalf("GIS upstream calls = %d, want only the authorized roster request", upstreamCalls)
	}
}

func TestGameSessionConsentRouteKeepsVoiceJWTAuthentication(t *testing.T) {
	var authorization, forwardedPath string
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		authorization, forwardedPath = r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims:   map[string]tokenClaims{"player-token": {UserID: "participant-account", ProfileID: "participant-profile"}},
		restUpstreams: map[string]http.Handler{"game-integrations": upstream},
	})
	path := "/api/v1/sessions/11111111-1111-4111-8111-111111111111/consent"
	rec := performRequest(h, http.MethodPost, path, `{"operation_id":"22222222-2222-4222-8222-222222222222","roster_revision":3}`, map[string]string{
		"Authorization": "Bearer player-token",
	})
	if rec.Code != http.StatusOK || authorization != "Bearer player-token" || forwardedPath != path || upstreamCalls != 1 {
		t.Fatalf("consent did not preserve player JWT route: status=%d auth=%q path=%q calls=%d body=%q", rec.Code, authorization, forwardedPath, upstreamCalls, rec.Body.String())
	}
	if isGameIntegrationSessionCredentialRoute(http.MethodPost, path) {
		t.Fatal("participant consent must never be exempted for game-server credentials")
	}
}

func TestGameSessionKeepGroupRouteKeepsVoiceJWTAuthentication(t *testing.T) {
	var authorization, forwardedPath string
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		authorization, forwardedPath = r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims:   map[string]tokenClaims{"player-token": {UserID: "participant-account", ProfileID: "participant-profile"}},
		restUpstreams: map[string]http.Handler{"game-integrations": upstream},
	})
	path := "/api/v1/sessions/11111111-1111-4111-8111-111111111111/keep-group"
	rec := performRequest(h, http.MethodPost, path, `{"operation_id":"22222222-2222-4222-8222-222222222222","roster_revision":3}`, map[string]string{
		"Authorization": "Bearer player-token",
	})
	if rec.Code != http.StatusOK || authorization != "Bearer player-token" || forwardedPath != path || upstreamCalls != 1 {
		t.Fatalf("keep-group did not preserve player JWT route: status=%d auth=%q path=%q calls=%d body=%q", rec.Code, authorization, forwardedPath, upstreamCalls, rec.Body.String())
	}
	if isGameIntegrationSessionCredentialRoute(http.MethodPost, path) {
		t.Fatal("participant keep-group must never be exempted for game-server credentials")
	}
	denied := performRequest(h, http.MethodPost, path, `{"operation_id":"22222222-2222-4222-8222-222222222222","roster_revision":3}`, map[string]string{
		"Authorization": "Bearer vgi1.game-server-credential",
	})
	if denied.Code != http.StatusUnauthorized || upstreamCalls != 1 {
		t.Fatalf("game-server credential must not access keep-group route: status=%d calls=%d", denied.Code, upstreamCalls)
	}
}

func TestGameCommunityRosterRouteUsesGISCredentialAndRoutesToGIS(t *testing.T) {
	var authorization, forwardedPath string
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		authorization, forwardedPath = r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	h := newGatewayForContract(t, gatewayTestOptions{
		restUpstreams: map[string]http.Handler{"game-integrations": upstream},
	})
	path := "/api/v1/community-bindings/corp-7/roster"
	credential := "vgi1.game-roster-credential"
	if namespace := restNamespace(path); namespace != "game-integrations" || !isPublicRESTNamespace(namespace) {
		t.Fatalf("community roster namespace = %q; route is not public", namespace)
	}
	if !isGameIntegrationSessionCredentialRoute(http.MethodPut, path) {
		t.Fatal("community roster must be exempt from player JWT auth and validated by GIS")
	}
	rec := performRequest(h, http.MethodPut, path, `{}`, map[string]string{"Authorization": "Bearer " + credential})
	if rec.Code != http.StatusOK || authorization != "Bearer "+credential || forwardedPath != path || upstreamCalls != 1 {
		t.Fatalf("community roster did not preserve GIS credential: status=%d auth=%q path=%q calls=%d body=%q", rec.Code, authorization, forwardedPath, upstreamCalls, rec.Body.String())
	}
	neighbor := performRequest(h, http.MethodPut, path+"/extra", `{}`, map[string]string{"Authorization": "Bearer " + credential})
	if neighbor.Code != http.StatusUnauthorized || upstreamCalls != 1 {
		t.Fatalf("neighboring route unexpectedly bypassed player auth: status=%d calls=%d", neighbor.Code, upstreamCalls)
	}
}

func TestGamePlayerProfileReadUsesVoiceJWTAndNeverAcceptsGameServerCredential(t *testing.T) {
	var authorization, forwardedPath string
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		authorization, forwardedPath = r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims:   map[string]tokenClaims{"player-token": {UserID: "player-account", ProfileID: "selected-profile"}},
		restUpstreams: map[string]http.Handler{"game-integrations": upstream},
	})
	path := "/api/v1/game-integrations/applications/11111111-1111-4111-8111-111111111111/environments/22222222-2222-4222-8222-222222222222/profiles/search?q=pilot"
	accepted := performRequest(h, http.MethodGet, path, "", map[string]string{"Authorization": "Bearer player-token"})
	if accepted.Code != http.StatusOK || authorization != "Bearer player-token" || forwardedPath != "/api/v1/game-integrations/applications/11111111-1111-4111-8111-111111111111/environments/22222222-2222-4222-8222-222222222222/profiles/search" || accepted.Header().Get("Cache-Control") != "private, no-store" || accepted.Header().Get("Pragma") != "no-cache" || !strings.Contains(accepted.Header().Get("Vary"), "Authorization") || upstreamCalls != 1 {
		t.Fatalf("player profile read did not preserve Voice JWT route: status=%d auth=%q path=%q calls=%d body=%q", accepted.Code, authorization, forwardedPath, upstreamCalls, accepted.Body.String())
	}
	denied := performRequest(h, http.MethodGet, path, "", map[string]string{"Authorization": "Bearer vgi1.game-server-credential"})
	if denied.Code != http.StatusUnauthorized || upstreamCalls != 1 {
		t.Fatalf("game-server credential must not access player profile reads: status=%d calls=%d", denied.Code, upstreamCalls)
	}
}

func TestGamePlayerSessionSnapshotPreservesVoiceJWTAuthentication(t *testing.T) {
	var authorization, forwardedPath string
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		authorization, forwardedPath = r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims:   map[string]tokenClaims{"player-token": {UserID: "participant-account", ProfileID: "participant-profile"}},
		restUpstreams: map[string]http.Handler{"game-integrations": upstream},
	})
	path := "/api/v1/sessions/11111111-1111-4111-8111-111111111111"
	rec := performRequest(h, http.MethodGet, path, "", map[string]string{
		"Authorization": "Bearer player-token",
	})
	if rec.Code != http.StatusOK || authorization != "Bearer player-token" || forwardedPath != path || upstreamCalls != 1 {
		t.Fatalf("player snapshot did not preserve Voice JWT route: status=%d auth=%q path=%q calls=%d body=%q", rec.Code, authorization, forwardedPath, upstreamCalls, rec.Body.String())
	}

	denied := performRequest(h, http.MethodGet, path, "", map[string]string{
		"Authorization": "Bearer vgi1.game-server-credential",
	})
	if denied.Code != http.StatusUnauthorized || upstreamCalls != 1 {
		t.Fatalf("game-server credential must not access player snapshot: status=%d calls=%d", denied.Code, upstreamCalls)
	}
}

func TestGamePlayerSessionListPreservesVoiceJWTAuthentication(t *testing.T) {
	var authorization, forwardedPath string
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		authorization, forwardedPath = r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"player-token": {UserID: "participant-account", ProfileID: "participant-profile"}}, restUpstreams: map[string]http.Handler{"game-integrations": upstream}})
	path := "/api/v1/player/sessions/me"
	rec := performRequest(h, http.MethodGet, path, "", map[string]string{"Authorization": "Bearer player-token"})
	if rec.Code != http.StatusOK || authorization != "Bearer player-token" || forwardedPath != path || upstreamCalls != 1 {
		t.Fatalf("player session list did not preserve Voice JWT route: status=%d auth=%q path=%q calls=%d body=%q", rec.Code, authorization, forwardedPath, upstreamCalls, rec.Body.String())
	}
	denied := performRequest(h, http.MethodGet, path, "", map[string]string{"Authorization": "Bearer vgi1.game-server-credential"})
	if denied.Code != http.StatusUnauthorized || upstreamCalls != 1 {
		t.Fatalf("game-server credential must not access player session list: status=%d calls=%d", denied.Code, upstreamCalls)
	}
}
