package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

type stubFriendLister struct {
	ids map[string][]string
	err error
}

func (s stubFriendLister) ListFriendProfileIDs(_ context.Context, profileID string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.ids[profileID], nil
}

func TestPresenceWireForObservers_invisible(t *testing.T) {
	st, custom := presenceWireForObservers("invisible", "busy")
	require.Empty(t, st)
	require.Empty(t, custom)

	st, custom = presenceWireForObservers("dnd", "focus")
	require.Equal(t, "dnd", st)
	require.Equal(t, "focus", custom)
}

func TestDispatchPresenceChangeToFriends_fansOut(t *testing.T) {
	hub := newWSHub()
	friendReg := hub.attachConn("i1", "c-friend", "friend-1", 8)
	t.Cleanup(func() { hub.unregisterConn(friendReg) })

	friends := stubFriendLister{ids: map[string][]string{
		"actor-1": {"friend-1", "offline-friend"},
	}}
	dispatchPresenceChangeToFriends(hub, friends, "actor-1", "dnd", nil, "")

	select {
	case env := <-friendReg.fanout:
		require.Equal(t, "presence_update", env.Op)
		var d map[string]any
		require.NoError(t, json.Unmarshal(env.D, &d))
		require.Equal(t, "actor-1", d["profile_id"])
		require.Equal(t, "dnd", d["status"])
	case <-time.After(2 * time.Second):
		t.Fatal("expected friend presence_update")
	}
}

func TestDispatchPresenceChangeToFriends_invisibleLooksOffline(t *testing.T) {
	hub := newWSHub()
	friendReg := hub.attachConn("i1", "c-friend", "friend-1", 8)
	t.Cleanup(func() { hub.unregisterConn(friendReg) })

	friends := stubFriendLister{ids: map[string][]string{"actor-1": {"friend-1"}}}
	dispatchPresenceChangeToFriends(hub, friends, "actor-1", "invisible", nil, "")

	select {
	case env := <-friendReg.fanout:
		var d map[string]any
		require.NoError(t, json.Unmarshal(env.D, &d))
		require.Equal(t, "actor-1", d["profile_id"])
		require.Equal(t, "", d["status"])
	case <-time.After(2 * time.Second):
		t.Fatal("expected friend presence_update")
	}
}

func TestRunUserEventsConsumer_JetStreamToFriendHub(t *testing.T) {
	opts := &server.Options{Port: -1, JetStream: true, StoreDir: t.TempDir()}
	ns, err := server.NewServer(opts)
	require.NoError(t, err)
	go ns.Start()
	t.Cleanup(ns.Shutdown)
	require.True(t, ns.ReadyForConnections(5*time.Second))

	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     jsStreamUserEvents,
		Subjects: []string{"user.presence_changed"},
	})
	require.NoError(t, err)

	hub := newWSHub()
	friendReg := hub.attachConn("i1", "c-friend", "friend-1", 8)
	t.Cleanup(func() { hub.unregisterConn(friendReg) })

	friends := stubFriendLister{ids: map[string][]string{"actor-1": {"friend-1"}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runUserEventsConsumer(ctx, hub, friends, ns.ClientURL(), "test-user-evt", nil)
	}()

	// Wait until durable consumer exists.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := js.ConsumerInfo(jsStreamUserEvents, userConsumerDurableName("test-user-evt")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("consumer not ready")
		}
		time.Sleep(50 * time.Millisecond)
	}

	env := &eventsv1.UserStreamEvent{
		EventId:    "evt-1",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.UserStreamEvent_PresenceChange{
			PresenceChange: &eventsv1.PresenceChange{
				ProfileId: "actor-1",
				Status:    "online",
			},
		},
	}
	b, err := proto.Marshal(env)
	require.NoError(t, err)
	_, err = js.Publish("user.presence_changed", b)
	require.NoError(t, err)

	select {
	case fe := <-friendReg.fanout:
		require.Equal(t, "presence_update", fe.Op)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for friend fanout")
	}
	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
	}
}
