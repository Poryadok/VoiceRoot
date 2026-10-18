package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStagingMediaFixtureBindsTargetAndDistinctParticipants(t *testing.T) {
	valid := stagingMediaFixture{
		Schema:        "voice-staging-media-v1",
		APIOrigin:     "https://api.example.test",
		LivekitOrigin: "wss://livekit.example.test",
		ChatID:        "11111111-1111-4111-8111-111111111111",
		Caller:        stagingMediaPeer{AccountID: "44444444-4444-4444-8444-444444444444", ProfileID: "22222222-2222-4222-8222-222222222222", AccessToken: "private-caller-token"},
		Callee:        stagingMediaPeer{AccountID: "55555555-5555-4555-8555-555555555555", ProfileID: "33333333-3333-4333-8333-333333333333", AccessToken: "private-callee-token"},
	}
	raw, _ := json.Marshal(valid)
	if _, err := parseStagingMediaFixture(raw, valid.APIOrigin); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
	for name, change := range map[string]func(*stagingMediaFixture){
		"wrong schema":      func(f *stagingMediaFixture) { f.Schema = "other" },
		"other target":      func(f *stagingMediaFixture) { f.APIOrigin = "https://other.example.test" },
		"plaintext API":     func(f *stagingMediaFixture) { f.APIOrigin = "http://api.example.test" },
		"API path":          func(f *stagingMediaFixture) { f.APIOrigin += "/api" },
		"API credentials":   func(f *stagingMediaFixture) { f.APIOrigin = "https://private-caller-token@api.example.test" },
		"plaintext media":   func(f *stagingMediaFixture) { f.LivekitOrigin = "ws://livekit.example.test" },
		"media query":       func(f *stagingMediaFixture) { f.LivekitOrigin += "?token=private-caller-token" },
		"missing chat":      func(f *stagingMediaFixture) { f.ChatID = "" },
		"missing token":     func(f *stagingMediaFixture) { f.Caller.AccessToken = "" },
		"header injection":  func(f *stagingMediaFixture) { f.Caller.AccessToken += "\r\nX-Header: value" },
		"duplicate profile": func(f *stagingMediaFixture) { f.Callee.ProfileID = f.Caller.ProfileID },
		"duplicate account": func(f *stagingMediaFixture) { f.Callee.AccountID = f.Caller.AccountID },
		"duplicate token":   func(f *stagingMediaFixture) { f.Callee.AccessToken = f.Caller.AccessToken },
	} {
		t.Run(name, func(t *testing.T) {
			fixture := valid
			change(&fixture)
			raw, _ := json.Marshal(fixture)
			_, err := parseStagingMediaFixture(raw, valid.APIOrigin)
			if err == nil {
				t.Fatal("unsafe fixture accepted")
			}
			if strings.Contains(err.Error(), "private-") {
				t.Fatal("error disclosed a credential")
			}
		})
	}
}

func TestStagingMediaProfilePreflightRejectsWrongSessionAndUpstreamFailures(t *testing.T) {
	peer := stagingMediaPeer{AccountID: "account-a", ProfileID: "profile-a", AccessToken: "private-test-token"}
	for _, tc := range []struct {
		name, body string
		status     int
		ok         bool
	}{
		{"matching", `{"profile":{"id":"profile-a","account_id":"account-a"}}`, 200, true},
		{"different profile", `{"profile":{"id":"profile-b","account_id":"account-a"}}`, 200, false},
		{"different account", `{"profile":{"id":"profile-a","account_id":"account-b"}}`, 200, false},
		{"unavailable", `private-test-token`, 503, false},
		{"malformed", `private-test-token`, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/users/me" || r.Header.Get("Authorization") != "Bearer "+peer.AccessToken {
					t.Error("unexpected preflight request")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			err := checkStagingMediaProfile(server.Client(), server.URL, peer)
			if (err == nil) != tc.ok {
				t.Fatal("wrong preflight decision")
			}
			if err != nil && strings.Contains(err.Error(), "private-") {
				t.Fatal("error disclosed credentials")
			}
		})
	}
}

func TestStagingMediaFixtureRejectsAmbiguousOrOversizedJSON(t *testing.T) {
	for _, raw := range []string{`{"unknown":"private-secret"}`, `{}`, `{} {}`, strings.Repeat(" ", 65537)} {
		if _, err := parseStagingMediaFixture([]byte(raw), "https://api.example.test"); err == nil {
			t.Fatal("invalid fixture accepted")
		}
	}
}
