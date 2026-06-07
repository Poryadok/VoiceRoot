package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	voicejwt "voice/backend/pkg/jwt"
)

func testRealtimeHandler(tv tokenValidator, lister dmChatLister) http.Handler {
	return newServiceHandler(serviceName, tv, lister, newWSHub(), nil, "test-instance")
}

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
	srv := httptest.NewServer(testRealtimeHandler(nil, nil))
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
	h := testRealtimeHandler(staticTokenValidator{"tok": {UserID: "a", ProfileID: "p"}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestWSRequiresAuthorization(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{"tok": {UserID: "a", ProfileID: "p"}}, nil))
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
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, nil))
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
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, nil))
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

func assertHelloConnID(t *testing.T, d json.RawMessage) {
	t.Helper()
	var payload map[string]string
	if err := json.Unmarshal(d, &payload); err != nil {
		t.Fatalf("hello d json: %v", err)
	}
	connID := payload["conn_id"]
	if connID == "" {
		t.Fatal("hello missing conn_id")
	}
	if _, err := uuid.Parse(connID); err != nil {
		t.Fatalf("hello conn_id = %q, want uuid: %v", connID, err)
	}
}

func TestWSHelloSequenceHeartbeatResume(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, nil))
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
	assertHelloConnID(t, hello.D)

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
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, nil))
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

type stubDMChatLister struct {
	ids []string
	err error
}

func (s stubDMChatLister) ListDMChatIDs(ctx context.Context, _, _ string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]string(nil), s.ids...), nil
}

type captureDMChatLister struct {
	gotAccount string
	gotProfile string
	ret        []string
}

func (c *captureDMChatLister) ListDMChatIDs(ctx context.Context, accountID, profileID string) ([]string, error) {
	c.gotAccount = accountID
	c.gotProfile = profileID
	return append([]string(nil), c.ret...), nil
}

func TestWSSendsDMSubscriptionSyncFromChatLister(t *testing.T) {
	t.Parallel()
	lister := stubDMChatLister{ids: []string{"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, lister))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
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

	_, data, err = c.ReadMessage()
	if err != nil {
		t.Fatalf("read subscription_sync: %v", err)
	}
	var sync wsEnvelope
	if err := json.Unmarshal(data, &sync); err != nil {
		t.Fatalf("subscription_sync json: %v", err)
	}
	if sync.Op != "subscription_sync" || sync.S != 2 {
		t.Fatalf("subscription_sync = %+v", sync)
	}
	var body struct {
		Scope    string   `json:"scope"`
		ChatIDs  []string `json:"chat_ids"`
		Source   string   `json:"source"`
		Degraded bool     `json:"degraded"`
	}
	if err := json.Unmarshal(sync.D, &body); err != nil {
		t.Fatalf("subscription_sync d: %v", err)
	}
	if body.Scope != "dm" || body.Source != "chat" || body.Degraded {
		t.Fatalf("unexpected body: %+v", body)
	}
	want := []string{
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
	}
	if len(body.ChatIDs) != len(want) {
		t.Fatalf("chat_ids = %v, want %v", body.ChatIDs, want)
	}
	for i := range want {
		if body.ChatIDs[i] != want[i] {
			t.Fatalf("chat_ids = %v, want sorted %v", body.ChatIDs, want)
		}
	}
}

func TestWSSendsDMSubscriptionSyncDegradedWhenChatListerFails(t *testing.T) {
	t.Parallel()
	lister := stubDMChatLister{err: errors.New("chat unavailable")}
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, lister))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	_, _, _ = c.ReadMessage() // hello
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read subscription_sync: %v", err)
	}
	var sync wsEnvelope
	if err := json.Unmarshal(data, &sync); err != nil {
		t.Fatalf("subscription_sync json: %v", err)
	}
	var body struct {
		ChatIDs  []string `json:"chat_ids"`
		Degraded bool     `json:"degraded"`
	}
	if err := json.Unmarshal(sync.D, &body); err != nil {
		t.Fatalf("subscription_sync d: %v", err)
	}
	if !body.Degraded || len(body.ChatIDs) != 0 {
		t.Fatalf("unexpected degraded body: %+v", body)
	}
}

func TestWSPassesAccountAndProfileToChatLister(t *testing.T) {
	t.Parallel()
	l := &captureDMChatLister{ret: []string{}}
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, l))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_, _, _ = c.ReadMessage()
	_, _, _ = c.ReadMessage()

	if l.gotAccount != "account-1" || l.gotProfile != "profile-1" {
		t.Fatalf("lister got account=%q profile=%q", l.gotAccount, l.gotProfile)
	}
}

func TestWSSubscribeACKForValidUUID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, nil))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_, _, _ = c.ReadMessage() // hello

	chatID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read subscribe_ack: %v", err)
	}
	var ack wsEnvelope
	if err := json.Unmarshal(data, &ack); err != nil {
		t.Fatalf("ack json: %v", err)
	}
	if ack.Op != "subscribe_ack" || ack.S != 2 {
		t.Fatalf("subscribe_ack = %+v", ack)
	}
	var body struct {
		ChatID string `json:"chat_id"`
	}
	if err := json.Unmarshal(ack.D, &body); err != nil {
		t.Fatalf("ack d: %v", err)
	}
	if body.ChatID != chatID {
		t.Fatalf("chat_id = %q, want %q", body.ChatID, chatID)
	}
}

func TestWSErrorOnInvalidSubscribeChatID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, nil))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_, _, _ = c.ReadMessage()

	if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": "not-a-uuid"}}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	var env wsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("error json: %v", err)
	}
	if env.Op != "error" || env.S != 2 {
		t.Fatalf("error env = %+v", env)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(env.D, &body); err != nil {
		t.Fatalf("error d: %v", err)
	}
	if body.Code != "invalid_subscribe" {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestWSUnsubscribeACK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, nil))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_, _, _ = c.ReadMessage()

	chatID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	if err := c.WriteJSON(map[string]any{"op": "unsubscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read unsubscribe_ack: %v", err)
	}
	var ack wsEnvelope
	if err := json.Unmarshal(data, &ack); err != nil {
		t.Fatalf("ack json: %v", err)
	}
	if ack.Op != "unsubscribe_ack" || ack.S != 2 {
		t.Fatalf("unsubscribe_ack = %+v", ack)
	}
}

func TestWSTypingFanoutTwoConnections(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	v := staticTokenValidator{
		"t1": {UserID: "a", ProfileID: "p1"},
		"t2": {UserID: "b", ProfileID: "p2"},
	}
	srv := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub, nil, "test-instance"))
	t.Cleanup(srv.Close)

	chatID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	dial := func(token, profile string) *websocket.Conn {
		t.Helper()
		u := wsEndpoint(t, srv)
		h := wsUpgradeHeaders(token)
		h.Set("X-Profile-Id", profile)
		c, _, err := websocket.DefaultDialer.Dial(u, h)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return c
	}

	c1 := dial("t1", "p1")
	t.Cleanup(func() { _ = c1.Close() })
	c2 := dial("t2", "p2")
	t.Cleanup(func() { _ = c2.Close() })

	readOp := func(c *websocket.Conn) wsEnvelope {
		t.Helper()
		_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var env wsEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatalf("json: %v", err)
		}
		return env
	}

	if op := readOp(c1); op.Op != "hello" {
		t.Fatalf("c1 first = %+v", op)
	}
	if op := readOp(c2); op.Op != "hello" {
		t.Fatalf("c2 first = %+v", op)
	}

	sub := map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}
	if err := c1.WriteJSON(sub); err != nil {
		t.Fatalf("c1 subscribe: %v", err)
	}
	if err := c2.WriteJSON(sub); err != nil {
		t.Fatalf("c2 subscribe: %v", err)
	}
	if op := readOp(c1); op.Op != "subscribe_ack" {
		t.Fatalf("c1 subscribe_ack = %+v", op)
	}
	if op := readOp(c2); op.Op != "subscribe_ack" {
		t.Fatalf("c2 subscribe_ack = %+v", op)
	}

	if err := c1.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("typing_start: %v", err)
	}

	typing := readOp(c2)
	if typing.Op != "typing" || typing.S != 3 {
		t.Fatalf("typing = %+v", typing)
	}
	var td struct {
		ChatID    string `json:"chat_id"`
		ProfileID string `json:"profile_id"`
		Kind      string `json:"kind"`
	}
	if err := json.Unmarshal(typing.D, &td); err != nil {
		t.Fatalf("typing d: %v", err)
	}
	if td.ChatID != chatID || td.ProfileID != "p1" || td.Kind != "start" {
		t.Fatalf("typing body = %+v", td)
	}
}

func TestWSTypingThrottleAndIdleStop(t *testing.T) {
	oldThrottle := typingThrottle
	oldIdle := typingIdleTimeout
	typingThrottle = 120 * time.Millisecond
	typingIdleTimeout = 180 * time.Millisecond
	t.Cleanup(func() {
		typingThrottle = oldThrottle
		typingIdleTimeout = oldIdle
	})

	chatID := "77777777-7777-4777-8777-777777777777"
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok1": {UserID: "a1", ProfileID: "p1"},
		"tok2": {UserID: "a2", ProfileID: "p2"},
	}, nil))
	t.Cleanup(srv.Close)
	u := wsEndpoint(t, srv)
	h1 := wsUpgradeHeaders("tok1")
	h1.Set("X-Profile-Id", "p1")
	c1, _, err := websocket.DefaultDialer.Dial(u, h1)
	if err != nil {
		t.Fatalf("dial c1: %v", err)
	}
	t.Cleanup(func() { _ = c1.Close() })
	h2 := wsUpgradeHeaders("tok2")
	h2.Set("X-Profile-Id", "p2")
	c2, _, err := websocket.DefaultDialer.Dial(u, h2)
	if err != nil {
		t.Fatalf("dial c2: %v", err)
	}
	t.Cleanup(func() { _ = c2.Close() })
	read := func(c *websocket.Conn) wsEnvelope {
		t.Helper()
		_ = c.SetReadDeadline(time.Now().Add(time.Second))
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var env wsEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatalf("json: %v", err)
		}
		return env
	}
	_ = read(c1)
	_ = read(c2)
	sub := map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}
	if err := c1.WriteJSON(sub); err != nil {
		t.Fatal(err)
	}
	if err := c2.WriteJSON(sub); err != nil {
		t.Fatal(err)
	}
	_ = read(c1)
	_ = read(c2)

	if err := c1.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatal(err)
	}
	start := read(c2)
	if start.Op != "typing" {
		t.Fatalf("start op = %+v", start)
	}
	if err := c1.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(typingIdleTimeout + 40*time.Millisecond)
	stop := read(c2)
	if stop.Op != "typing" {
		t.Fatalf("stop op = %+v", stop)
	}
	var body struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(stop.D, &body); err != nil {
		t.Fatalf("stop body: %v", err)
	}
	if body.Kind != "stop" {
		t.Fatalf("kind = %q, want stop", body.Kind)
	}
}

func TestWSTypingRejectedWithoutSubscription(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(testRealtimeHandler(staticTokenValidator{
		"tok": {UserID: "account-1", ProfileID: "profile-1"},
	}, nil))
	t.Cleanup(srv.Close)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_, _, _ = c.ReadMessage()

	chatID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	if err := c.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("typing_start: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env wsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("json: %v", err)
	}
	if env.Op != "error" {
		t.Fatalf("want error, got %+v", env)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(env.D, &body); err != nil {
		t.Fatalf("d: %v", err)
	}
	if body.Code != "invalid_typing" {
		t.Fatalf("code = %q", body.Code)
	}
}
