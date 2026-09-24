package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
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

type retryActivityStore struct {
	calls atomic.Int32
	done  chan struct{}
}

func (s *retryActivityStore) TouchLastMessageAt(context.Context, uuid.UUID, time.Time) error {
	if s.calls.Add(1) == 1 {
		return errors.New("transient write failure")
	}
	return nil
}
func (s *retryActivityStore) PromoteDeclinedDMRecipients(context.Context, uuid.UUID, uuid.UUID) error {
	select {
	case s.done <- struct{}{}:
	default:
	}
	return nil
}

func TestMessageActivityFromEvent_MessageSent(t *testing.T) {
	chatID := uuid.New()
	at := time.Date(2026, 6, 3, 1, 2, 3, 0, time.UTC)
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(at),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       uuid.NewString(),
				ChatId:          chatID.String(),
				SenderProfileId: uuid.NewString(),
			},
		},
	}
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	gotChat, gotSender, gotAt, ok := messageActivityFromEvent(data, func() time.Time {
		return at.Add(time.Hour)
	})
	if !ok {
		t.Fatal("expected message.sent activity")
	}
	if gotChat != chatID {
		t.Fatalf("chat = %s, want %s", gotChat, chatID)
	}
	if gotSender.String() != env.GetMessageSent().GetSenderProfileId() {
		t.Fatalf("sender = %s", gotSender)
	}
	if !gotAt.Equal(at) {
		t.Fatalf("at = %s, want %s", gotAt, at)
	}
}

func TestMessageActivityFromEvent_RequiresSenderProfileID(t *testing.T) {
	chatID := uuid.New()
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId: uuid.NewString(),
				ChatId:    chatID.String(),
			},
		},
	}
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := messageActivityFromEvent(data, time.Now); ok {
		t.Fatal("message.sent without sender_profile_id must be skipped")
	}
}

func TestMessageActivityFromEvent_SkipsInvalidPayloads(t *testing.T) {
	if _, _, _, ok := messageActivityFromEvent([]byte{0x1, 0x2}, time.Now); ok {
		t.Fatal("invalid protobuf must be skipped")
	}
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{MessageId: uuid.NewString(), ChatId: uuid.NewString()},
		},
	}
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := messageActivityFromEvent(data, time.Now); ok {
		t.Fatal("non-sent events must be skipped for activity updates")
	}
}

func TestChatActivityDurableName(t *testing.T) {
	require.Equal(t, "chat_message_activity", chatActivityDurableName("pod-a"))
	require.Equal(t, chatActivityDurableName("pod-a"), chatActivityDurableName("pod-b"))
}

func TestMessageActivityRequiresExactPreprovisionedDurable(t *testing.T) {
	s := startAccountDeletedJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: messageEventsStreamName, Subjects: []string{"message.sent"}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	require.Error(t, validateMessageActivityDurable(js))
	config := messageActivityConsumerConfig()
	config.FilterSubject = ""
	_, err = js.AddConsumer(messageEventsStreamName, config)
	require.NoError(t, err)
	require.Error(t, validateMessageActivityDurable(js))
}

func TestMessageActivityBindsPreprovisionedQueue(t *testing.T) {
	s := startAccountDeletedJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: messageEventsStreamName, Subjects: []string{messageActivitySubject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	_, err = js.AddConsumer(messageEventsStreamName, messageActivityConsumerConfig())
	require.NoError(t, err)
	store := &archivePreservingActivityStore{}
	subA, err := subscribeMessageActivity(context.Background(), js, store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subA.Unsubscribe() })
	subB, err := subscribeMessageActivity(context.Background(), js, store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subB.Unsubscribe() })
}

func TestMessageActivityRetriesAfterHandlerFailureWithTwoReplicas(t *testing.T) {
	s := startAccountDeletedJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: messageEventsStreamName, Subjects: []string{messageActivitySubject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	config := messageActivityConsumerConfig()
	config.AckWait = 200 * time.Millisecond
	_, err = js.AddConsumer(messageEventsStreamName, config)
	require.NoError(t, err)
	store := &retryActivityStore{done: make(chan struct{}, 2)}
	subA, err := subscribeMessageActivity(context.Background(), js, store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subA.Unsubscribe() })
	subB, err := subscribeMessageActivity(context.Background(), js, store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subB.Unsubscribe() })
	payload, err := proto.Marshal(&eventsv1.MessageStreamEvent{
		Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: uuid.NewString(), SenderProfileId: uuid.NewString()}},
	})
	require.NoError(t, err)
	_, err = js.Publish(messageActivitySubject, payload)
	require.NoError(t, err)
	select {
	case <-store.done:
	case <-time.After(5 * time.Second):
		t.Fatal("message was not redelivered")
	}
	require.GreaterOrEqual(t, store.calls.Load(), int32(2))
	require.NoError(t, subA.Unsubscribe())
	_, err = js.Publish(messageActivitySubject, payload)
	require.NoError(t, err)
	select {
	case <-store.done:
	case <-time.After(5 * time.Second):
		t.Fatal("surviving replica did not receive next message")
	}
}

func TestMessageActivityBindsWithoutConsumerCreatePermission(t *testing.T) {
	s, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(),
		Users: []*server.User{
			{Username: "admin", Password: "admin"},
			{Username: "chat", Password: "chat", Permissions: &server.Permissions{
				Publish:   &server.SubjectPermission{Allow: []string{"$JS.API.CONSUMER.INFO.message_events.chat_message_activity", "$JS.ACK.message_events.chat_message_activity.>"}},
				Subscribe: &server.SubjectPermission{Allow: []string{"_INBOX.voice.chat.>"}},
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
	_, err = adminJS.AddStream(&nats.StreamConfig{Name: messageEventsStreamName, Subjects: []string{messageActivitySubject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	_, err = adminJS.AddConsumer(messageEventsStreamName, messageActivityConsumerConfig())
	require.NoError(t, err)
	url := "nats://chat:chat@" + strings.TrimPrefix(s.ClientURL(), "nats://")
	nc, err := nats.Connect(url, nats.CustomInboxPrefix("_INBOX.voice.chat"))
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	sub, err := subscribeMessageActivity(context.Background(), js, &archivePreservingActivityStore{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
}
