package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func startRealtimeJSTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	t.Cleanup(func() { s.Shutdown() })
	return s
}

func TestMessageEventBytesToFanout_SentEditedDeleted(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	sender := uuid.NewString()

	sent := &eventsv1.MessageStreamEvent{
		EventId:    "e1",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: sender,
			},
		},
	}
	b, err := proto.Marshal(sent)
	if err != nil {
		t.Fatal(err)
	}
	gotChat, fe, ok := messageEventBytesToFanout(b)
	if !ok || gotChat != chatID {
		t.Fatalf("sent: ok=%v chat=%q", ok, gotChat)
	}
	if fe.Op != "message_create" {
		t.Fatalf("op=%q", fe.Op)
	}
	var d map[string]any
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID || d["sender_profile_id"] != sender {
		t.Fatalf("d=%v", d)
	}

	edited := &eventsv1.MessageStreamEvent{
		EventId:    "e2",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{MessageId: msgID, ChatId: chatID},
		},
	}
	b, _ = proto.Marshal(edited)
	gotChat, fe, ok = messageEventBytesToFanout(b)
	if !ok || fe.Op != "message_update" {
		t.Fatalf("edited: ok=%v op=%q", ok, fe.Op)
	}
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID {
		t.Fatalf("edited d=%v", d)
	}

	deleted := &eventsv1.MessageStreamEvent{
		EventId:    "e3",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageDeleted{
			MessageDeleted: &eventsv1.MessageDeleted{MessageId: msgID, ChatId: chatID},
		},
	}
	b, _ = proto.Marshal(deleted)
	gotChat, fe, ok = messageEventBytesToFanout(b)
	if !ok || fe.Op != "message_delete" {
		t.Fatalf("deleted: ok=%v op=%q", ok, fe.Op)
	}
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID {
		t.Fatalf("deleted d=%v", d)
	}
}

func TestMessageEventBytesToFanout_ReactionSkipped(t *testing.T) {
	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: uuid.NewString(),
				ProfileId: uuid.NewString(),
				Emoji:     "👍",
			},
		},
	}
	b, _ := proto.Marshal(ev)
	_, _, ok := messageEventBytesToFanout(b)
	if ok {
		t.Fatal("expected reaction to be skipped for WS fan-out")
	}
}

func TestConsumerDurableName_ReplacesHyphensInInstanceID(t *testing.T) {
	if got := consumerDurableName("a-b-c"); got != "rt_abc_msg" {
		t.Fatalf("got %q", got)
	}
	if got := consumerDurableName(""); got != "rt_unknown_msg" {
		t.Fatalf("empty instance: got %q", got)
	}
}

func TestRunMessageEventsConsumer_ErrorsWhenHubOrNATSURLMissing(t *testing.T) {
	ctx := context.Background()
	if err := runMessageEventsConsumer(ctx, nil, "nats://127.0.0.1:4222", "x"); err == nil {
		t.Fatal("expected error for nil hub")
	}
	hub := newWSHub()
	if err := runMessageEventsConsumer(ctx, hub, "", "x"); err == nil {
		t.Fatal("expected error for empty NATS URL")
	}
	if err := runMessageEventsConsumer(ctx, hub, "   ", "x"); err == nil {
		t.Fatal("expected error for whitespace NATS URL")
	}
}

func TestMessageEventBytesToFanout_InvalidOrIncompletePayload(t *testing.T) {
	_, _, ok := messageEventBytesToFanout([]byte{0x01, 0x02})
	if ok {
		t.Fatal("invalid protobuf should be skipped")
	}

	sentNil := &eventsv1.MessageStreamEvent{
		EventId:    "e",
		OccurredAt: timestamppb.Now(),
		Payload:    &eventsv1.MessageStreamEvent_MessageSent{MessageSent: nil},
	}
	b, _ := proto.Marshal(sentNil)
	_, _, ok = messageEventBytesToFanout(b)
	if ok {
		t.Fatal("nil MessageSent should be skipped")
	}

	sentNoChat := &eventsv1.MessageStreamEvent{
		EventId:    "e2",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: ""},
		},
	}
	b, _ = proto.Marshal(sentNoChat)
	_, _, ok = messageEventBytesToFanout(b)
	if ok {
		t.Fatal("empty chat_id should be skipped")
	}
}

func TestRunMessageEventsConsumer_JetStreamToHub(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	natsURL := s.ClientURL()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      "message_events",
		Subjects:  []string{"message.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	chatID := "44444444-4444-4444-4444-444444444444"
	msgID := uuid.NewString()
	sender := uuid.NewString()

	hub := newWSHub()
	reg := hub.attachConn("test-inst", "conn-1", "", 8)
	hub.addChat(reg, chatID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runMessageEventsConsumer(ctx, hub, natsURL, "test-consumer-inst") }()
	time.Sleep(200 * time.Millisecond)

	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: sender,
			},
		},
	}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("message.sent", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case fe := <-reg.fanout:
		if fe.Op != "message_create" {
			t.Fatalf("op=%q", fe.Op)
		}
		var d map[string]any
		if err := json.Unmarshal(fe.D, &d); err != nil {
			t.Fatal(err)
		}
		if d["message_id"] != msgID || d["chat_id"] != chatID {
			t.Fatalf("payload=%v", d)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timeout waiting for fan-out")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("consumer exit: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not exit")
	}
}
