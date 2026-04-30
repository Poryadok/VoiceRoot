package main

import (
	"io"
	"net/http"
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
