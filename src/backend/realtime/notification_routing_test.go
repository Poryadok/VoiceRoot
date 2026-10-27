package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/notificationrouting"
)

func TestNotificationRoutingRealNATSDeliversUnsubscribedMemberToEveryTab(t *testing.T) {
	server := startRealtimeJSTestServer(t)
	nc, e := nats.Connect(server.ClientURL())
	require.NoError(t, e)
	defer nc.Close()
	chat, message, sender, recipient, denied := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	data, e := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chat, MessageId: message, SenderProfileId: sender}}})
	require.NoError(t, e)
	_, e = nc.QueueSubscribe(notificationrouting.Subject, notificationrouting.Queue, func(m *nats.Msg) {
		wire, err := notificationrouting.Encode(m.Data, []notificationrouting.Target{{ProfileID: recipient, Type: "new_message"}}, false)
		if err == nil {
			_ = m.Respond(wire)
		}
	})
	require.NoError(t, e)
	require.NoError(t, nc.Flush())
	hub := newWSHub()
	hub.setNotificationRouter(natsMessageNotificationRouter{connection: nc})
	tabA := hub.attachConn("instance", "tab-a", recipient, 8)
	tabB := hub.attachConn("instance", "tab-b", recipient, 8)
	deniedTab := hub.attachConn("instance", "denied", denied, 8)
	hub.addChat(deniedTab, chat)
	dispatchMessageStreamEvent(hub, data, nil, nil, "")
	for _, tab := range []*connReg{tabA, tabB} {
		select {
		case got := <-tab.fanout:
			require.Equal(t, "new_message", notificationPayload(t, got)["type"])
		case <-time.After(time.Second):
			t.Fatal("unsubscribed member missed notification")
		}
	}
	for len(deniedTab.fanout) > 0 {
		require.NotEqual(t, "notification", (<-deniedTab.fanout).Op)
	}
	// Policy availability is mandatory even for an already subscribed profile.
	nc.Close()
	dispatchMessageStreamEvent(hub, data, nil, nil, "")
	require.Empty(t, tabA.fanout)
	require.Empty(t, tabB.fanout)
	for len(deniedTab.fanout) > 0 {
		require.NotEqual(t, "notification", (<-deniedTab.fanout).Op)
	}
}

type routingTargets []notificationrouting.Target

// Consumer fixtures explicitly provide the owner decision over real NATS.
// Notification's separate boundary suite proves current policy/authorization.
func bindNotificationDecisionFixture(t *testing.T, nc *nats.Conn, profileID, typ string) {
	t.Helper()
	sub, e := nc.QueueSubscribe(notificationrouting.Subject, notificationrouting.Queue, func(m *nats.Msg) {
		wire, err := notificationrouting.Encode(m.Data, []notificationrouting.Target{{ProfileID: profileID, Type: typ}}, false)
		if err == nil {
			_ = m.Respond(wire)
		}
	})
	require.NoError(t, e)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	require.NoError(t, nc.Flush())
}

func (r routingTargets) Route(context.Context, []byte) ([]notificationrouting.Target, error) {
	return r, nil
}
func TestNotificationRoutingRejectsDecisionForDifferentEventType(t *testing.T) {
	sender, recipient := uuid.NewString(), uuid.NewString()
	data, e := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: uuid.NewString(), MessageId: uuid.NewString(), SenderProfileId: sender}}})
	require.NoError(t, e)
	got, e := notificationPolicyFanouts(context.Background(), routingTargets{{ProfileID: recipient, Type: "mention"}}, data)
	require.Error(t, e)
	require.Empty(t, got)
}
