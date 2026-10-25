package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/pushenrich"
	"voice/backend/notification/internal/store"
)

func TestChatQuotaActualConsumerRedisFailureNaksRecoveryAndSuppressionAcks(t *testing.T) {
	server, err := natsserver.NewServer(&natsserver.Options{JetStream: true, StoreDir: t.TempDir(), Port: -1})
	require.NoError(t, err)
	go server.Start()
	require.True(t, server.ReadyForConnections(10*time.Second))
	defer server.Shutdown()
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	defer nc.Close()
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: jsStreamMessageEvents, Subjects: []string{jsSubjectMessageEvents}})
	require.NoError(t, err)
	_, err = js.AddConsumer(jsStreamMessageEvents, &nats.ConsumerConfig{Durable: consumer.SharedDurable("message"), FilterSubject: jsSubjectMessageEvents, DeliverSubject: "_INBOX.voice.notification.message", AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy})
	require.NoError(t, err)
	redisServer := miniredis.RunT(t)
	redisServer.SetError("ERR unavailable")
	env := map[string]string{"NATS_URL": server.ClientURL(), "NOTIFICATION_REDIS_ADDR": redisServer.Addr(), "NOTIFICATION_CHAT_PUSH_LIMIT": "1"}
	quota, err := newChatPushAdmission(func(k string) (string, bool) { v, ok := env[k]; return v, ok }, prometheus.NewRegistry())
	require.NoError(t, err)
	defer func() { _ = quota.Close() }()
	sender, recipient, chat := uuid.New(), uuid.New(), uuid.NewString()
	sent := make(chan push.Payload, 4)
	pusher := &dispatch.MessagePusher{Admission: quota, Presence: offlineMessagePresence{}, GameChatScope: ordinaryGameChatScope{}, Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipient: {{Token: "one", PushService: "fcm"}}}}, Pusher: &dispatch.PushDispatcher{FCM: channelMessageFCM{sent: sent}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readiness := newNotificationConsumerReadiness("message")
	done := make(chan error, 1)
	go func() {
		done <- runMessageEventsConsumer(withNotificationConsumerReadiness(ctx, readiness, "message"), server.ClientURL(), &store.DeviceTokenStore{}, stubChatMembers{ids: []string{sender.String(), recipient.String()}}, pusher, pushenrich.NoopResolver{}, nil)
	}()
	require.Eventually(t, readiness.ready, 5*time.Second, 10*time.Millisecond)
	publish := func() {
		raw, err := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: chat, SenderProfileId: sender.String()}}})
		require.NoError(t, err)
		_, err = js.Publish("message.sent", raw)
		require.NoError(t, err)
	}
	publish()
	require.Eventually(t, func() bool {
		info, err := js.ConsumerInfo(jsStreamMessageEvents, consumer.SharedDurable("message"))
		return err == nil && info.NumRedelivered > 0
	}, 8*time.Second, 20*time.Millisecond, "Redis error must cause real durable redelivery")
	require.Empty(t, sent, "failed ordinary attempts never call provider")
	redisServer.SetError("")
	select {
	case <-sent:
	case <-time.After(8 * time.Second):
		t.Fatal("recovery did not dispatch admitted message")
	}
	settled := func() bool {
		info, err := js.ConsumerInfo(jsStreamMessageEvents, consumer.SharedDurable("message"))
		return err == nil && info.NumPending == 0 && info.NumAckPending == 0
	}
	require.Eventually(t, settled, 5*time.Second, 20*time.Millisecond)
	publish()
	require.Eventually(t, settled, 5*time.Second, 20*time.Millisecond)
	require.Empty(t, sent, "quota denial ACKs with no provider effect")
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not terminate")
	}
}
