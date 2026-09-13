package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
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

// stubPresenceViewer is the User-service seam expected by the privacy-aware
// fan-out. It returns the already filtered snapshot for exactly one viewer.
// Realtime must not derive a recipient's view from the event payload.
type stubPresenceViewer struct {
	mu       sync.Mutex
	byViewer map[string]viewerPresence
	errs     map[string]error
	calls    []presenceViewerCall
}

type presenceViewerCall struct {
	targetProfileID   string
	viewerProfileID   string
	viewerAccountType string
}

func (s *stubPresenceViewer) PresenceForViewer(_ context.Context, targetProfileID, viewerProfileID, viewerAccountType string) (viewerPresence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, presenceViewerCall{
		targetProfileID:   targetProfileID,
		viewerProfileID:   viewerProfileID,
		viewerAccountType: viewerAccountType,
	})
	if err := s.errs[viewerProfileID]; err != nil {
		return viewerPresence{}, err
	}
	return s.byViewer[viewerProfileID], nil
}

func (s *stubPresenceViewer) Calls() []presenceViewerCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]presenceViewerCall(nil), s.calls...)
}

func mustPresencePayload(t *testing.T, env fanoutEnvelope) map[string]any {
	t.Helper()
	require.Equal(t, "presence_update", env.Op)
	var d map[string]any
	require.NoError(t, json.Unmarshal(env.D, &d))
	return d
}

func requireNoPresenceFanout(t *testing.T, reg *connReg) {
	t.Helper()
	select {
	case env := <-reg.fanout:
		t.Fatalf("unexpected presence fan-out: %+v", env)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestDispatchPresenceChangeToFriends_fansOut(t *testing.T) {
	hub := newWSHub()
	friendReg := hub.attachConn("i1", "c-friend", "friend-1", 8)
	t.Cleanup(func() { hub.unregisterConn(friendReg) })

	friends := stubFriendLister{ids: map[string][]string{
		"actor-1": {"friend-1", "offline-friend"},
	}}
	viewer := &stubPresenceViewer{byViewer: map[string]viewerPresence{
		"friend-1":       {Status: "dnd", CustomStatus: "focus"},
		"offline-friend": {Status: "dnd", CustomStatus: "focus"},
	}}
	dispatchPresenceChangeToFriends(hub, friends, viewer, "actor-1", "dnd", nil, "")

	select {
	case env := <-friendReg.fanout:
		d := mustPresencePayload(t, env)
		require.Equal(t, "actor-1", d["profile_id"])
		require.Equal(t, "dnd", d["status"])
		require.Equal(t, "focus", d["custom_status"])
	case <-time.After(2 * time.Second):
		t.Fatal("expected friend presence_update")
	}
	require.Equal(t, []presenceViewerCall{
		{targetProfileID: "actor-1", viewerProfileID: "friend-1", viewerAccountType: "regular"},
	}, viewer.Calls())
}

func TestDispatchPresenceChangeToFriends_appliesViewerSnapshotPerFriend(t *testing.T) {
	hub := newWSHub()
	deniedReg := hub.attachConn("i1", "c-denied", "friend-denied", 8)
	lastSeenDeniedReg := hub.attachConn("i1", "c-last-seen-denied", "friend-last-seen-denied", 8)
	allowedReg := hub.attachConn("i1", "c-allowed", "friend-allowed", 8)
	t.Cleanup(func() {
		hub.unregisterConn(deniedReg)
		hub.unregisterConn(lastSeenDeniedReg)
		hub.unregisterConn(allowedReg)
	})

	friends := stubFriendLister{ids: map[string][]string{"actor-1": {"friend-denied", "friend-last-seen-denied", "friend-allowed"}}}
	lastSeen := time.Date(2026, time.September, 13, 10, 11, 12, 0, time.UTC)
	viewer := &stubPresenceViewer{byViewer: map[string]viewerPresence{
		// show_online denied: User returns the offline sparse snapshot.
		"friend-denied": {},
		// show_online allowed but show_last_seen denied: status remains online.
		"friend-last-seen-denied": {Status: "online", CustomStatus: "coding"},
		// show_online allowed: all fields that User permitted reach this viewer.
		"friend-allowed": {Status: "online", CustomStatus: "coding", LastSeen: &lastSeen},
	}}
	// The source event says dnd. Each recipient must receive its User-filtered
	// snapshot instead of this raw event status.
	dispatchPresenceChangeToFriends(hub, friends, viewer, "actor-1", "dnd", nil, "")

	select {
	case env := <-deniedReg.fanout:
		d := mustPresencePayload(t, env)
		require.Equal(t, "actor-1", d["profile_id"])
		require.NotContains(t, d, "status")
		require.NotContains(t, d, "custom_status")
		require.NotContains(t, d, "last_seen")
	case <-time.After(2 * time.Second):
		t.Fatal("expected show_online-denied presence_update")
	}
	select {
	case env := <-lastSeenDeniedReg.fanout:
		d := mustPresencePayload(t, env)
		require.Equal(t, "actor-1", d["profile_id"])
		require.Equal(t, "online", d["status"])
		require.Equal(t, "coding", d["custom_status"])
		require.NotContains(t, d, "last_seen")
	case <-time.After(2 * time.Second):
		t.Fatal("expected show_last_seen-denied presence_update")
	}
	select {
	case env := <-allowedReg.fanout:
		d := mustPresencePayload(t, env)
		require.Equal(t, "actor-1", d["profile_id"])
		require.Equal(t, "online", d["status"])
		require.Equal(t, "coding", d["custom_status"])
		require.Equal(t, lastSeen.Format(time.RFC3339), d["last_seen"])
	case <-time.After(2 * time.Second):
		t.Fatal("expected show_last_seen-allowed presence_update")
	}
	require.ElementsMatch(t, []presenceViewerCall{
		{targetProfileID: "actor-1", viewerProfileID: "friend-denied", viewerAccountType: "regular"},
		{targetProfileID: "actor-1", viewerProfileID: "friend-last-seen-denied", viewerAccountType: "regular"},
		{targetProfileID: "actor-1", viewerProfileID: "friend-allowed", viewerAccountType: "regular"},
	}, viewer.Calls())
}

func TestDispatchPresenceChangeToFriends_invisibleLooksOffline(t *testing.T) {
	hub := newWSHub()
	friendReg := hub.attachConn("i1", "c-friend", "friend-1", 8)
	t.Cleanup(func() { hub.unregisterConn(friendReg) })

	friends := stubFriendLister{ids: map[string][]string{"actor-1": {"friend-1"}}}
	viewer := &stubPresenceViewer{byViewer: map[string]viewerPresence{
		// User must already mask invisible as an offline, timestamp-free snapshot.
		"friend-1": {},
	}}
	dispatchPresenceChangeToFriends(hub, friends, viewer, "actor-1", "invisible", nil, "")

	select {
	case env := <-friendReg.fanout:
		d := mustPresencePayload(t, env)
		require.Equal(t, "actor-1", d["profile_id"])
		require.NotContains(t, d, "status")
		require.NotContains(t, d, "custom_status")
		require.NotContains(t, d, "last_seen")
	case <-time.After(2 * time.Second):
		t.Fatal("expected invisible presence_update")
	}
}

func TestDispatchPresenceChangeToFriends_policyErrorFailsClosed(t *testing.T) {
	hub := newWSHub()
	friendReg := hub.attachConn("i1", "c-friend", "friend-1", 8)
	t.Cleanup(func() { hub.unregisterConn(friendReg) })

	friends := stubFriendLister{ids: map[string][]string{"actor-1": {"friend-1"}}}
	viewer := &stubPresenceViewer{errs: map[string]error{"friend-1": errors.New("user presence unavailable")}}
	dispatchPresenceChangeToFriends(hub, friends, viewer, "actor-1", "online", nil, "")

	requireNoPresenceFanout(t, friendReg)
}

func TestBroadcastPrivatePresenceInChatsExcept_deduplicatesSharedRecipients(t *testing.T) {
	hub := newWSHub()
	sender := hub.attachConn("i1", "c-sender", "actor-1", 8)
	recipient := hub.attachConn("i1", "c-recipient", "friend-1", 8)
	t.Cleanup(func() {
		hub.unregisterConn(sender)
		hub.unregisterConn(recipient)
	})
	firstChat := "11111111-1111-1111-1111-111111111111"
	secondChat := "22222222-2222-2222-2222-222222222222"
	for _, chatID := range []string{firstChat, secondChat} {
		require.True(t, hub.addChat(sender, chatID))
		require.True(t, hub.addChat(recipient, chatID))
	}
	viewer := &stubPresenceViewer{byViewer: map[string]viewerPresence{
		"friend-1": {Status: "online"},
	}}
	hub.setPresenceViewer(viewer)

	hub.broadcastPrivatePresenceInChatsExcept([]string{secondChat, firstChat}, "actor-1", "online", "i1", "c-sender", nil)

	var d map[string]any
	select {
	case env := <-recipient.fanout:
		d = mustPresencePayload(t, env)
	case <-time.After(2 * time.Second):
		t.Fatal("expected one deduplicated presence_update")
	}
	require.Equal(t, firstChat, d["chat_id"], "first canonical chat gives the recipient one deterministic event")
	require.Equal(t, []presenceViewerCall{{targetProfileID: "actor-1", viewerProfileID: "friend-1", viewerAccountType: "regular"}}, viewer.Calls())
	requireNoPresenceFanout(t, recipient)
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
	viewer := &stubPresenceViewer{byViewer: map[string]viewerPresence{"friend-1": {Status: "online"}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runUserEventsConsumer(ctx, hub, friends, viewer, ns.ClientURL(), "test-user-evt", nil)
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
