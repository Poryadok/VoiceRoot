package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/releasepolicy"
)

func TestReleasePolicyWSRejectsCustomStatusBeforeOwnerAndFanout(t *testing.T) {
	data, err := os.ReadFile("../pkg/releasepolicy/testdata/free-invite-alpha.json")
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	policy, err := releasepolicy.Parse(data, hex.EncodeToString(sum[:]), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	hub := newWSHub()
	hub.releasePolicy = policy
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	sub := rdb.Subscribe(context.Background(), "release-effects")
	t.Cleanup(func() { _ = sub.Close() })
	_, err = sub.Receive(context.Background())
	require.NoError(t, err)
	rf := newRedisFanout(redisFanoutConfig{Client: rdb, Hub: hub, InstanceID: "release-test", FanoutChannel: "release-effects"})
	updater := newRecordingPresenceUpdater(10)
	srv := httptest.NewServer(newServiceHandlerWithPresence(serviceName, staticTokenValidator{"tok": {UserID: "owner", ProfileID: "profile"}}, nil, hub, rf, "release-test", updater, nil, readinessDeps{}))
	t.Cleanup(srv.Close)
	dial := func() *websocket.Conn {
		h := wsUpgradeHeaders("tok")
		h.Set("X-Profile-Id", "profile")
		c, _, err := websocket.DefaultDialer.Dial(wsEndpoint(t, srv), h)
		require.NoError(t, err)
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	read := func(c *websocket.Conn) wsEnvelope {
		_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := c.ReadMessage()
		require.NoError(t, err)
		var env wsEnvelope
		require.NoError(t, json.Unmarshal(data, &env))
		return env
	}
	a, b := dial(), dial()
	require.Equal(t, "hello", read(a).Op)
	require.Equal(t, "hello", read(b).Op)
	// Wait for both initial owner calls, which occur after hello. This avoids
	// mistaking a late initial call for an effect of the denied request.
	for range 2 {
		updater.expect(t, recordedPresenceUpdate{accountID: "owner", profileID: "profile", status: "online"})
	}
	require.NoError(t, a.WriteJSON(map[string]any{"op": "presence_update", "d": map[string]any{"status": "dnd", "custom_status": "blocked"}}))
	// Error response is the processing barrier, not a sleep-based no-effects check.
	env := read(a)
	require.Equal(t, "error", env.Op)
	var denied map[string]any
	require.NoError(t, json.Unmarshal(env.D, &denied))
	require.Equal(t, "capability_unavailable", denied["code"])
	require.Empty(t, updater.updates)
	// A permitted control must be the observer's first frame; a leaked denied
	// fanout would precede it and fail these exact body assertions.
	require.NoError(t, a.WriteJSON(map[string]any{"op": "presence_update", "d": map[string]any{"status": "idle", "custom_status": ""}}))
	env = read(b)
	require.Equal(t, "presence_update", env.Op)
	var control map[string]any
	require.NoError(t, json.Unmarshal(env.D, &control))
	require.Equal(t, "idle", control["status"])
	require.Equal(t, "", control["custom_status"])
	updater.expect(t, recordedPresenceUpdate{accountID: "owner", profileID: "profile", status: "idle", customStatus: ""})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	publication, err := sub.ReceiveMessage(ctx)
	require.NoError(t, err)
	var effect redisFanoutPayload
	require.NoError(t, json.Unmarshal([]byte(publication.Payload), &effect))
	require.Equal(t, "idle", effect.Status)
	require.Empty(t, effect.CustomStatus)
	require.NoError(t, a.WriteJSON(map[string]any{"op": "message.send", "d": map[string]any{}}))
	env = read(a)
	require.Equal(t, "error", env.Op)
	require.NoError(t, json.Unmarshal(env.D, &denied))
	require.Equal(t, "unknown_operation", denied["code"])
}

type rejectingCustomPresenceUpdater struct{ *recordingPresenceUpdater }

func (u rejectingCustomPresenceUpdater) UpdatePresence(ctx context.Context, account, profile, state, custom string) error {
	if custom != "" {
		return status.Error(codes.FailedPrecondition, "custom status requires premium subscription")
	}
	return u.recordingPresenceUpdater.UpdatePresence(ctx, account, profile, state, custom)
}

func TestReleasePolicyWSOwnerFailureDoesNotFanout(t *testing.T) {
	hub := newWSHub()
	updater := newRecordingPresenceUpdater(10)
	srv := httptest.NewServer(newServiceHandlerWithPresence(serviceName, staticTokenValidator{"tok": {UserID: "owner", ProfileID: "profile"}}, nil, hub, nil, "owner-deny", rejectingCustomPresenceUpdater{updater}, nil, readinessDeps{}))
	t.Cleanup(srv.Close)
	dial := func() *websocket.Conn {
		h := wsUpgradeHeaders("tok")
		h.Set("X-Profile-Id", "profile")
		c, _, err := websocket.DefaultDialer.Dial(wsEndpoint(t, srv), h)
		require.NoError(t, err)
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	read := func(c *websocket.Conn) wsEnvelope {
		_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := c.ReadMessage()
		require.NoError(t, err)
		var env wsEnvelope
		require.NoError(t, json.Unmarshal(data, &env))
		return env
	}
	a, b := dial(), dial()
	require.Equal(t, "hello", read(a).Op)
	require.Equal(t, "hello", read(b).Op)
	for range 2 {
		updater.expect(t, recordedPresenceUpdate{accountID: "owner", profileID: "profile", status: "online"})
	}
	require.NoError(t, a.WriteJSON(map[string]any{"op": "presence_update", "d": map[string]any{"status": "dnd", "custom_status": "denied-by-user"}}))
	env := read(a)
	require.Equal(t, "error", env.Op)
	var body map[string]any
	require.NoError(t, json.Unmarshal(env.D, &body))
	require.Equal(t, "capability_unavailable", body["code"])
	require.NoError(t, a.WriteJSON(map[string]any{"op": "presence_update", "d": map[string]any{"status": "idle", "custom_status": ""}}))
	env = read(b)
	require.Equal(t, "presence_update", env.Op)
	require.NoError(t, json.Unmarshal(env.D, &body))
	require.Equal(t, "idle", body["status"])
	require.Equal(t, "", body["custom_status"])
}
