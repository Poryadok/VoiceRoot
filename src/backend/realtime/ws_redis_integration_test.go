package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func TestWSTypingCrossInstancesViaRedis(t *testing.T) {
	s := miniredis.RunT(t)
	t.Cleanup(s.Close)
	addr := s.Addr()

	chName := "voice:rt:test:wsredis:" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub1 := newWSHub()
	hub2 := newWSHub()
	rsub1 := redis.NewClient(&redis.Options{Addr: addr})
	rsub2 := redis.NewClient(&redis.Options{Addr: addr})
	rchk := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = rsub1.Close()
		_ = rsub2.Close()
		_ = rchk.Close()
	})

	v := staticTokenValidator{
		"t1": {UserID: "a", ProfileID: "p1"},
		"t2": {UserID: "b", ProfileID: "p2"},
	}

	pfx1 := "voice:rt:test:wsredis1:"
	pfx2 := "voice:rt:test:wsredis2:"
	rf1 := newRedisFanout(redisFanoutConfig{
		Client:        rsub1,
		Hub:           hub1,
		InstanceID:    "i1",
		KeyPrefix:     pfx1,
		FanoutChannel: chName,
	})
	rf2 := newRedisFanout(redisFanoutConfig{
		Client:        rsub2,
		Hub:           hub2,
		InstanceID:    "i2",
		KeyPrefix:     pfx2,
		FanoutChannel: chName,
	})
	go func() { _ = rf1.runSubscriber(ctx) }()
	go func() { _ = rf2.runSubscriber(ctx) }()
	time.Sleep(100 * time.Millisecond)

	srv1 := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub1, rf1, "i1"))
	srv2 := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub2, rf2, "i2"))
	t.Cleanup(srv1.Close)
	t.Cleanup(srv2.Close)

	chatID := "33333333-3333-3333-3333-333333333333"

	dial := func(srv *httptest.Server, token, profile string) *websocket.Conn {
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

	c1 := dial(srv1, "t1", "p1")
	t.Cleanup(func() { _ = c1.Close() })
	c2 := dial(srv2, "t2", "p2")
	t.Cleanup(func() { _ = c2.Close() })

	deadline := time.Now().Add(3 * time.Second)
	var found bool
	for time.Now().Before(deadline) {
		mem, err := rchk.SMembers(context.Background(), pfx1+"p1").Result()
		if err == nil && len(mem) > 0 && strings.HasPrefix(mem[0], "i1:") {
			found = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !found {
		t.Fatal("expected profile registry entry for p1 on instance i1")
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

	if readOp(c1).Op != "hello" {
		t.Fatal("c1 expected hello")
	}
	if readOp(c2).Op != "hello" {
		t.Fatal("c2 expected hello")
	}

	sub := map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}
	if err := c1.WriteJSON(sub); err != nil {
		t.Fatalf("c1 subscribe: %v", err)
	}
	if err := c2.WriteJSON(sub); err != nil {
		t.Fatalf("c2 subscribe: %v", err)
	}
	if readOp(c1).Op != "subscribe_ack" {
		t.Fatal("c1 subscribe_ack")
	}
	if readOp(c2).Op != "subscribe_ack" {
		t.Fatal("c2 subscribe_ack")
	}

	if err := c1.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("typing_start: %v", err)
	}

	typing := readOp(c2)
	if typing.Op != "typing" {
		t.Fatalf("c2 expected typing, got %+v", typing)
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

func TestWSMarkReadCrossInstancesViaRedis(t *testing.T) {
	s := miniredis.RunT(t)
	t.Cleanup(s.Close)
	addr := s.Addr()

	chName := "voice:rt:test:wsredis-mr:" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub1 := newWSHub()
	hub2 := newWSHub()
	rsub1 := redis.NewClient(&redis.Options{Addr: addr})
	rsub2 := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = rsub1.Close()
		_ = rsub2.Close()
	})

	v := staticTokenValidator{
		"mobile":  {UserID: "u1", ProfileID: "prof-same"},
		"desktop": {UserID: "u1", ProfileID: "prof-same"},
	}

	pfx1 := "voice:rt:test:wsredis-mr1:"
	pfx2 := "voice:rt:test:wsredis-mr2:"
	rf1 := newRedisFanout(redisFanoutConfig{
		Client:        rsub1,
		Hub:           hub1,
		InstanceID:    "mr-i1",
		KeyPrefix:     pfx1,
		FanoutChannel: chName,
	})
	rf2 := newRedisFanout(redisFanoutConfig{
		Client:        rsub2,
		Hub:           hub2,
		InstanceID:    "mr-i2",
		KeyPrefix:     pfx2,
		FanoutChannel: chName,
	})
	go func() { _ = rf1.runSubscriber(ctx) }()
	go func() { _ = rf2.runSubscriber(ctx) }()
	time.Sleep(100 * time.Millisecond)

	srv1 := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub1, rf1, "mr-i1"))
	srv2 := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub2, rf2, "mr-i2"))
	t.Cleanup(srv1.Close)
	t.Cleanup(srv2.Close)

	chatID := "10101010-1010-1010-1010-101010101010"
	msgID := "20202020-2020-2020-2020-202020202020"

	dial := func(srv *httptest.Server, token string) *websocket.Conn {
		t.Helper()
		u := wsEndpoint(t, srv)
		h := wsUpgradeHeaders(token)
		h.Set("X-Profile-Id", "prof-same")
		c, _, err := websocket.DefaultDialer.Dial(u, h)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return c
	}

	c1 := dial(srv1, "mobile")
	t.Cleanup(func() { _ = c1.Close() })
	c2 := dial(srv2, "desktop")
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
		"d":  map[string]any{"chat_id": chatID, "message_id": msgID},
	}); err != nil {
		t.Fatalf("mark_read: %v", err)
	}

	ev := readOp(c2)
	if ev.Op != "mark_read" {
		t.Fatalf("c2 expected mark_read, got %+v", ev)
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

func TestWSProfilePresenceCrossInstancesViaRedis(t *testing.T) {
	s := miniredis.RunT(t)
	t.Cleanup(s.Close)
	addr := s.Addr()

	chName := "voice:rt:test:wsredis-pr:" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub1 := newWSHub()
	hub2 := newWSHub()
	rsub1 := redis.NewClient(&redis.Options{Addr: addr})
	rsub2 := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = rsub1.Close()
		_ = rsub2.Close()
	})

	v := staticTokenValidator{
		"mobile":  {UserID: "u1", ProfileID: "prof-same"},
		"desktop": {UserID: "u1", ProfileID: "prof-same"},
	}

	pfx1 := "voice:rt:test:wsredis-pr1:"
	pfx2 := "voice:rt:test:wsredis-pr2:"
	rf1 := newRedisFanout(redisFanoutConfig{
		Client:        rsub1,
		Hub:           hub1,
		InstanceID:    "pr-i1",
		KeyPrefix:     pfx1,
		FanoutChannel: chName,
	})
	rf2 := newRedisFanout(redisFanoutConfig{
		Client:        rsub2,
		Hub:           hub2,
		InstanceID:    "pr-i2",
		KeyPrefix:     pfx2,
		FanoutChannel: chName,
	})
	go func() { _ = rf1.runSubscriber(ctx) }()
	go func() { _ = rf2.runSubscriber(ctx) }()
	time.Sleep(100 * time.Millisecond)

	srv1 := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub1, rf1, "pr-i1"))
	srv2 := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub2, rf2, "pr-i2"))
	t.Cleanup(srv1.Close)
	t.Cleanup(srv2.Close)

	dial := func(srv *httptest.Server, token string) *websocket.Conn {
		t.Helper()
		u := wsEndpoint(t, srv)
		h := wsUpgradeHeaders(token)
		h.Set("X-Profile-Id", "prof-same")
		c, _, err := websocket.DefaultDialer.Dial(u, h)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return c
	}

	c1 := dial(srv1, "mobile")
	t.Cleanup(func() { _ = c1.Close() })
	c2 := dial(srv2, "desktop")
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

	if readOp(c1).Op != "hello" || readOp(c2).Op != "hello" {
		t.Fatal("expected hello on both")
	}

	if err := c1.WriteJSON(map[string]any{
		"op": "presence_update",
		"d":  map[string]any{"status": "online", "custom_status": "from-mobile"},
	}); err != nil {
		t.Fatalf("presence_update: %v", err)
	}

	ev := readOp(c2)
	if ev.Op != "presence_update" {
		t.Fatalf("c2 expected presence_update, got %+v", ev)
	}
	var body struct {
		ProfileID    string `json:"profile_id"`
		Status       string `json:"status"`
		CustomStatus string `json:"custom_status"`
		ChatID       string `json:"chat_id"`
	}
	if err := json.Unmarshal(ev.D, &body); err != nil {
		t.Fatalf("d: %v", err)
	}
	if body.ProfileID != "prof-same" || body.Status != "online" || body.CustomStatus != "from-mobile" {
		t.Fatalf("presence body = %+v", body)
	}
	if body.ChatID != "" {
		t.Fatalf("profile-scope fan-out should omit chat_id, got %q", body.ChatID)
	}
}
