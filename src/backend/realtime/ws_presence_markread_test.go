package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// presence_update / mark_read WebSocket behavior — see docs/ARCHITECTURE_REQUIREMENTS.md,
// docs/microservices/realtime-service.md (outbound presence_update).

func TestWSMarkReadFanoutSameProfileTwoConnections(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	v := staticTokenValidator{
		"mobile":  {UserID: "u1", ProfileID: "prof-same"},
		"desktop": {UserID: "u1", ProfileID: "prof-same"},
	}
	srv := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub, nil, "test-instance"))
	t.Cleanup(srv.Close)

	chatID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	msgID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

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

	c1 := dial("mobile", "prof-same")
	t.Cleanup(func() { _ = c1.Close() })
	c2 := dial("desktop", "prof-same")
	t.Cleanup(func() { _ = c2.Close() })

	if readOp(c1).Op != "hello" || readOp(c2).Op != "hello" {
		t.Fatal("expected hello on both")
	}

	sub := map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}
	if err := c1.WriteJSON(sub); err != nil {
		t.Fatalf("c1 subscribe: %v", err)
	}
	if err := c2.WriteJSON(sub); err != nil {
		t.Fatalf("c2 subscribe: %v", err)
	}
	if readOp(c1).Op != "subscribe_ack" || readOp(c2).Op != "subscribe_ack" {
		t.Fatal("subscribe_ack expected")
	}

	if err := c1.WriteJSON(map[string]any{
		"op": "mark_read",
		"d": map[string]any{"chat_id": chatID, "message_id": msgID},
	}); err != nil {
		t.Fatalf("mark_read: %v", err)
	}

	ev := readOp(c2)
	if ev.Op != "mark_read" {
		t.Fatalf("want mark_read, got %+v", ev)
	}
	var body struct {
		ChatID    string `json:"chat_id"`
		MessageID string `json:"message_id"`
		ProfileID string `json:"profile_id"`
	}
	if err := json.Unmarshal(ev.D, &body); err != nil {
		t.Fatalf("d: %v", err)
	}
	if body.ChatID != chatID || body.MessageID != msgID || body.ProfileID != "prof-same" {
		t.Fatalf("mark_read body = %+v", body)
	}
}

func TestWSMarkReadRejectedWithoutChatSubscription(t *testing.T) {
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

	chatID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	msgID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	if err := c.WriteJSON(map[string]any{
		"op": "mark_read",
		"d": map[string]any{"chat_id": chatID, "message_id": msgID},
	}); err != nil {
		t.Fatalf("mark_read: %v", err)
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
	var errBody struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(env.D, &errBody); err != nil {
		t.Fatalf("d: %v", err)
	}
	if errBody.Code != "invalid_mark_read" {
		t.Fatalf("code = %q", errBody.Code)
	}
}

func TestWSPresenceUpdateFanoutToPeerInSharedChat(t *testing.T) {
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

	c1 := dial("t1", "p1")
	t.Cleanup(func() { _ = c1.Close() })
	c2 := dial("t2", "p2")
	t.Cleanup(func() { _ = c2.Close() })

	if readOp(c1).Op != "hello" || readOp(c2).Op != "hello" {
		t.Fatal("hello")
	}

	sub := map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}
	if err := c1.WriteJSON(sub); err != nil {
		t.Fatalf("c1 subscribe: %v", err)
	}
	if err := c2.WriteJSON(sub); err != nil {
		t.Fatalf("c2 subscribe: %v", err)
	}
	if readOp(c1).Op != "subscribe_ack" || readOp(c2).Op != "subscribe_ack" {
		t.Fatal("subscribe_ack")
	}

	if err := c1.WriteJSON(map[string]any{
		"op": "presence_update",
		"d": map[string]any{"status": "dnd", "custom_status": "in a meeting"},
	}); err != nil {
		t.Fatalf("presence_update: %v", err)
	}

	ev := readOp(c2)
	if ev.Op != "presence_update" {
		t.Fatalf("want presence_update, got %+v", ev)
	}
	var body struct {
		ChatID       string `json:"chat_id"`
		ProfileID    string `json:"profile_id"`
		Status       string `json:"status"`
		CustomStatus string `json:"custom_status"`
	}
	if err := json.Unmarshal(ev.D, &body); err != nil {
		t.Fatalf("d: %v", err)
	}
	if body.ChatID != chatID || body.ProfileID != "p1" || body.Status != "dnd" || body.CustomStatus != "in a meeting" {
		t.Fatalf("presence body = %+v", body)
	}
}

func TestWSPresenceUpdateRejectedEmptyStatus(t *testing.T) {
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
	if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	_, _, _ = c.ReadMessage()

	if err := c.WriteJSON(map[string]any{
		"op": "presence_update",
		"d": map[string]any{"status": ""},
	}); err != nil {
		t.Fatalf("presence_update: %v", err)
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
	var errBody struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(env.D, &errBody); err != nil {
		t.Fatalf("d: %v", err)
	}
	if errBody.Code != "invalid_presence" {
		t.Fatalf("code = %q", errBody.Code)
	}
}
