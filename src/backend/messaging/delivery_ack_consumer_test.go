package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

type deliveryStoreStub struct{}

func (deliveryStoreStub) UpsertDeliveredCursor(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}

func startMessagingJSTestServer(t *testing.T) *server.Server {
	t.Helper()
	s, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir()})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))
	t.Cleanup(s.Shutdown)
	return s
}

func TestMessagingConsumerContractsAreFixed(t *testing.T) {
	require.Equal(t, "messaging_delivery_ack", deliveryAckDurableName("pod-a"))
	require.Equal(t, deliveryAckDurableName("pod-a"), deliveryAckDurableName("pod-b"))
	require.Equal(t, "messaging_receipt_privacy", privacySettingsDurableName("pod-a"))
	require.Equal(t, privacySettingsDurableName("pod-a"), privacySettingsDurableName("pod-b"))
}

func TestDeliveryAckRequiresExactPreprovisionedDurable(t *testing.T) {
	s := startMessagingJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: deliveryAckStreamName, Subjects: []string{"message.delivery_ack"}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	require.Error(t, validateDeliveryAckDurable(js))
	config := deliveryAckConsumerConfig()
	config.DeliverGroup = "wrong_queue"
	_, err = js.AddConsumer(deliveryAckStreamName, config)
	require.NoError(t, err)
	require.Error(t, validateDeliveryAckDurable(js))
}

func TestDeliveryAckBindsPreprovisionedQueue(t *testing.T) {
	s := startMessagingJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: deliveryAckStreamName, Subjects: []string{deliveryAckSubject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	_, err = js.AddConsumer(deliveryAckStreamName, deliveryAckConsumerConfig())
	require.NoError(t, err)
	subA, err := subscribeDeliveryAck(context.Background(), js, deliveryStoreStub{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subA.Unsubscribe() })
	subB, err := subscribeDeliveryAck(context.Background(), js, deliveryStoreStub{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subB.Unsubscribe() })
}

func TestMessagingBindsWithoutConsumerCreatePermission(t *testing.T) {
	s, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(),
		Users: []*server.User{
			{Username: "admin", Password: "admin"},
			{Username: "messaging", Password: "messaging", Permissions: &server.Permissions{
				Publish: &server.SubjectPermission{Allow: []string{
					"$JS.API.CONSUMER.INFO.message_events.messaging_delivery_ack",
					"$JS.API.CONSUMER.INFO.user_events.messaging_receipt_privacy",
					"$JS.ACK.message_events.messaging_delivery_ack.>",
					"$JS.ACK.user_events.messaging_receipt_privacy.>",
				}},
				Subscribe: &server.SubjectPermission{Allow: []string{"_INBOX.voice.messaging.>"}},
			}},
		},
	})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))
	t.Cleanup(s.Shutdown)
	admin, err := nats.Connect(s.ClientURL(), nats.UserInfo("admin", "admin"))
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	adminJS, err := admin.JetStream()
	require.NoError(t, err)
	_, err = adminJS.AddStream(&nats.StreamConfig{Name: deliveryAckStreamName, Subjects: []string{deliveryAckSubject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	_, err = adminJS.AddStream(&nats.StreamConfig{Name: privacySettingsStreamName, Subjects: []string{privacySettingsSubject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	_, err = adminJS.AddConsumer(deliveryAckStreamName, deliveryAckConsumerConfig())
	require.NoError(t, err)
	_, err = adminJS.AddConsumer(privacySettingsStreamName, receiptPrivacyConsumerConfig())
	require.NoError(t, err)
	url := "nats://messaging:messaging@" + strings.TrimPrefix(s.ClientURL(), "nats://")
	nc, err := nats.Connect(url, nats.CustomInboxPrefix("_INBOX.voice.messaging"))
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	deliverySub, err := subscribeDeliveryAck(context.Background(), js, deliveryStoreStub{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = deliverySub.Unsubscribe() })
	privacySub, err := subscribeReceiptPrivacy(context.Background(), js, privacyStoreStub{}, privacyTargetsStub{}, privacyPublisherStub{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = privacySub.Unsubscribe() })
}

func TestDeliveryAckFromEvent(t *testing.T) {
	chatID := uuid.NewString()
	profileID := uuid.NewString()
	messageID := uuid.NewString()
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_DeliveryAck{
			DeliveryAck: &eventsv1.DeliveryAck{
				ChatId:    chatID,
				ProfileId: profileID,
				MessageId: messageID,
			},
		},
	}
	b, err := proto.Marshal(env)
	require.NoError(t, err)
	gotChat, gotProfile, gotMsg, ok := deliveryAckFromEvent(b)
	require.True(t, ok)
	require.Equal(t, chatID, gotChat.String())
	require.Equal(t, profileID, gotProfile.String())
	require.Equal(t, messageID, gotMsg.String())
}

func TestDeliveryAckFromEventRejectsNonDeliveryAndMalformedPayloads(t *testing.T) {
	validID := uuid.NewString()
	tests := []struct {
		name string
		env  *eventsv1.MessageStreamEvent
		raw  []byte
	}{
		{
			name: "other message event",
			env: &eventsv1.MessageStreamEvent{
				EventId: uuid.NewString(),
				Payload: &eventsv1.MessageStreamEvent_MessageRead{
					MessageRead: &eventsv1.MessageRead{ChatId: validID, ProfileId: validID, MessageId: validID},
				},
			},
		},
		{
			name: "malformed delivery acknowledgement id",
			env: &eventsv1.MessageStreamEvent{
				EventId: uuid.NewString(),
				Payload: &eventsv1.MessageStreamEvent_DeliveryAck{
					DeliveryAck: &eventsv1.DeliveryAck{ChatId: "not-a-uuid", ProfileId: validID, MessageId: validID},
				},
			},
		},
		{name: "invalid protobuf", raw: []byte("not-protobuf")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.raw
			if tt.env != nil {
				var err error
				raw, err = proto.Marshal(tt.env)
				require.NoError(t, err)
			}
			chatID, profileID, messageID, ok := deliveryAckFromEvent(raw)
			require.False(t, ok)
			require.Equal(t, uuid.Nil, chatID)
			require.Equal(t, uuid.Nil, profileID)
			require.Equal(t, uuid.Nil, messageID)
		})
	}
}
