package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestChatEventBytesToFanout_CreatedAndMemberChanged(t *testing.T) {
	chatID := uuid.NewString()
	profileID := uuid.NewString()

	created := &eventsv1.ChatStreamEvent{
		EventId:    "e1",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_ChatCreated{
			ChatCreated: &eventsv1.ChatCreated{ChatId: chatID, Type: "dm"},
		},
	}
	b, err := proto.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	pid, fe, ok := chatEventBytesToFanout(b)
	if !ok || pid != "" || fe.Op != "chat_update" {
		t.Fatalf("created: pid=%q ok=%v op=%q", pid, ok, fe.Op)
	}
	var d map[string]string
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["type"] != "dm" {
		t.Fatalf("created d=%v", d)
	}

	changed := &eventsv1.ChatStreamEvent{
		EventId:    "e2",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{
			ChatMemberChanged: &eventsv1.ChatMemberChanged{
				ChatId:    chatID,
				ProfileId: profileID,
				Change:    "joined",
			},
		},
	}
	b, _ = proto.Marshal(changed)
	pid, fe, ok = chatEventBytesToFanout(b)
	if !ok || pid != profileID || fe.Op != "chat_update" {
		t.Fatalf("member: pid=%q ok=%v op=%q", pid, ok, fe.Op)
	}
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["profile_id"] != profileID || d["change"] != "joined" {
		t.Fatalf("member d=%v", d)
	}
}

func TestChatConsumerDurableName_ReplacesHyphens(t *testing.T) {
	if got := chatConsumerDurableName("a-b"); got != "rt_ab_chat" {
		t.Fatalf("got %q", got)
	}
}

func TestRunChatEventsConsumer_ErrorsWhenHubOrNATSURLMissing(t *testing.T) {
	ctx := context.Background()
	if err := runChatEventsConsumer(ctx, nil, "nats://127.0.0.1:4222", "x", nil); err == nil {
		t.Fatal("expected error for nil hub")
	}
	hub := newWSHub()
	if err := runChatEventsConsumer(ctx, hub, "", "x", nil); err == nil {
		t.Fatal("expected error for empty NATS URL")
	}
}

func TestRunChatEventsConsumer_JetStreamToProfile(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	natsURL := s.ClientURL()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Drain() }()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      jsStreamChatEvents,
		Subjects:  []string{"chat.>"},
		Retention: nats.LimitsPolicy,
	})
	if err != nil {
		t.Fatal(err)
	}

	hub := newWSHub()
	profileID := uuid.NewString()
	chatID := uuid.NewString()
	reg := hub.attachConn("inst", "conn-1", profileID, 8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runChatEventsConsumer(ctx, hub, natsURL, "chat-test-inst", nil) }()
	time.Sleep(300 * time.Millisecond)

	changed := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{
			ChatMemberChanged: &eventsv1.ChatMemberChanged{
				ChatId:    chatID,
				ProfileId: profileID,
				Change:    "joined",
			},
		},
	}
	b, _ := proto.Marshal(changed)
	if _, err := js.Publish("chat.member_changed", b); err != nil {
		t.Fatal(err)
	}

	select {
	case env := <-reg.fanout:
		if env.Op != "chat_update" {
			t.Fatalf("op=%q", env.Op)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for chat_update fan-out")
	}
	cancel()
	<-errCh
}
