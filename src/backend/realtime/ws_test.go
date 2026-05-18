package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	voicejwt "voice/backend/pkg/jwt"
)

func wsEndpoint(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/ws"
	return u.String()
}

type staticTokenValidator map[string]voicejwt.Claims

func (v staticTokenValidator) Validate(r *http.Request) (voicejwt.Claims, string) {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return voicejwt.Claims{}, "invalid_token"
	}
	claims, ok := v[strings.TrimPrefix(auth, prefix)]
	if !ok {
		return voicejwt.Claims{}, "invalid_token"
	}
	return claims, ""
}

func wsUpgradeHeaders(token string) http.Header {
	// Only custom headers; gorilla/websocket.Dial sets Sec-WebSocket-* and Upgrade/Connection.
	h := http.Header{}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	return h
}

func TestWSReturns503WhenJWKSNotConfigured(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newServiceHandler(serviceName, nil))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "p")
	_, resp, err := websocket.DefaultDialer.Dial(u, hdr)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %v, want 503", resp)
	}
}

func TestWSRequiresWebSocketUpgrade(t *testing.T) {
	t.Parallel()
	h := newServiceHandler(serviceName, staticTokenValidator{"tok": {UserID: "a", ProfileID: "p"}})
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestWSRequiresAuthorization(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newServiceHandler(serviceName, staticTokenValidator{"tok": {UserID: "a", ProfileID: "p"}}))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	_, resp, err := websocket.DefaultDialer.Dial(u, wsUpgradeHeaders(""))
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestWSRequiresActiveProfileHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newServiceHandler(serviceName, staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	_, resp, err := websocket.DefaultDialer.Dial(u, hdr)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestWSRejectsProfileMismatch(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newServiceHandler(serviceName, staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "other-profile")
	_, resp, err := websocket.DefaultDialer.Dial(u, hdr)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

type wsEnvelope struct {
	Op string          `json:"op"`
	S  int64           `json:"s"`
	D  json.RawMessage `json:"d"`
}

func TestWSHelloSequenceHeartbeatResume(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newServiceHandler(serviceName, staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, resp, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v; status=%v", err, resp)
	}
	t.Cleanup(func() { _ = c.Close() })

	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read hello: %v", err)
	}
	var hello wsEnvelope
	if err := json.Unmarshal(data, &hello); err != nil {
		t.Fatalf("hello json: %v", err)
	}
	if hello.Op != "hello" || hello.S != 1 {
		t.Fatalf("hello = %+v", hello)
	}

	if err := c.WriteJSON(map[string]any{"op": "heartbeat"}); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}
	_, data, err = c.ReadMessage()
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	var ack wsEnvelope
	if err := json.Unmarshal(data, &ack); err != nil {
		t.Fatalf("ack json: %v", err)
	}
	if ack.Op != "heartbeat_ack" || ack.S != 2 {
		t.Fatalf("heartbeat_ack = %+v", ack)
	}

	if err := c.WriteJSON(map[string]any{"op": "resume", "d": map[string]any{"last_s": 2}}); err != nil {
		t.Fatalf("write resume: %v", err)
	}
	if err := c.WriteJSON(map[string]any{"op": "heartbeat"}); err != nil {
		t.Fatalf("write heartbeat after resume: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err = c.ReadMessage()
	if err != nil {
		t.Fatalf("read ack after resume: %v", err)
	}
	if err := json.Unmarshal(data, &ack); err != nil {
		t.Fatalf("ack json: %v", err)
	}
	if ack.Op != "heartbeat_ack" || ack.S != 3 {
		t.Fatalf("heartbeat_ack after resume = %+v", ack)
	}
}

func TestWSAcceptsGatewayProfileHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newServiceHandler(serviceName, staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Voice-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = c.Close()
}
