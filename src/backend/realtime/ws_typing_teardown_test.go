package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Expiry and disconnect must share ownership of the connection's timer state.
// Waiting for the real idle-stop frame exercises the callback before teardown;
// -race must also prove that teardown does not inspect its map unsynchronized.
func TestWSTypingExpiryThenDisconnect(t *testing.T) {
	oldIdle := typingIdleTimeout
	typingIdleTimeout = 10 * time.Millisecond
	t.Cleanup(func() { typingIdleTimeout = oldIdle })
	chatID := uuid.NewString()
	hub := permitAllTestSubscriptions(newWSHub())
	h := newServiceHandler(serviceName, staticTokenValidator{
		"writer": {UserID: "writer-account", ProfileID: "writer-profile"},
		"peer":   {UserID: "peer-account", ProfileID: "peer-profile"},
	}, perProfileBootstrapLister{"writer-profile": {chatID}, "peer-profile": {chatID}}, hub, nil, "typing-teardown", readinessDeps{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	peer := dialACLTestConn(t, srv, "peer", "peer-profile")
	require.Equal(t, "subscription_sync", readACLEnvelope(t, peer).Op)
	for i := 0; i < 8; i++ {
		writer := dialACLTestConn(t, srv, "writer", "writer-profile")
		require.Equal(t, "subscription_sync", readACLEnvelope(t, writer).Op)
		require.NoError(t, writer.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}))
		for _, kind := range []string{"start", "stop"} {
			env := readACLEnvelope(t, peer)
			require.Equal(t, "typing", env.Op)
			var body struct {
				Kind string `json:"kind"`
			}
			require.NoError(t, json.Unmarshal(env.D, &body))
			require.Equal(t, kind, body.Kind)
		}
		require.NoError(t, writer.Close())
		require.Eventually(t, func() bool {
			hub.mu.RLock()
			defer hub.mu.RUnlock()
			return len(hub.byProfile["writer-profile"]) == 0
		}, time.Second, time.Millisecond, "expired writer must complete teardown")
	}
}

func TestWSTypingDisconnectWithExpiringTimers(t *testing.T) {
	oldIdle := typingIdleTimeout
	typingIdleTimeout = time.Millisecond
	t.Cleanup(func() { typingIdleTimeout = oldIdle })
	chats := make([]string, 64)
	for i := range chats {
		chats[i] = uuid.NewString()
	}
	hub := permitAllTestSubscriptions(newWSHub())
	h := newServiceHandler(serviceName, staticTokenValidator{
		"writer": {UserID: "writer-account", ProfileID: "writer-profile"},
	}, perProfileBootstrapLister{"writer-profile": chats}, hub, nil, "typing-expiring-teardown", readinessDeps{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	for i := 0; i < 16; i++ {
		writer := dialACLTestConn(t, srv, "writer", "writer-profile")
		require.Equal(t, "subscription_sync", readACLEnvelope(t, writer).Op)
		for _, chatID := range chats {
			require.NoError(t, writer.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}))
		}
		// This barrier confirms the inbound loop has scheduled every timer.
		require.NoError(t, writer.WriteJSON(map[string]any{"op": "heartbeat", "d": map[string]any{}}))
		require.Equal(t, "heartbeat_ack", readACLEnvelope(t, writer).Op)
		require.NoError(t, writer.Close())
		require.Eventually(t, func() bool {
			hub.mu.RLock()
			defer hub.mu.RUnlock()
			return len(hub.byProfile["writer-profile"]) == 0
		}, time.Second, time.Millisecond, "disconnect must stop outstanding typing timers")
	}
}
