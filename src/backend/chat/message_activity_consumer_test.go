package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

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

	gotChat, gotAt, ok := messageActivityFromEvent(data, func() time.Time {
		return at.Add(time.Hour)
	})
	if !ok {
		t.Fatal("expected message.sent activity")
	}
	if gotChat != chatID {
		t.Fatalf("chat = %s, want %s", gotChat, chatID)
	}
	if !gotAt.Equal(at) {
		t.Fatalf("at = %s, want %s", gotAt, at)
	}
}

func TestMessageActivityFromEvent_SkipsInvalidPayloads(t *testing.T) {
	if _, _, ok := messageActivityFromEvent([]byte{0x1, 0x2}, time.Now); ok {
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
	if _, _, ok := messageActivityFromEvent(data, time.Now); ok {
		t.Fatal("non-sent events must be skipped for activity updates")
	}
}

func TestChatActivityDurableName(t *testing.T) {
	if got := chatActivityDurableName("a-b-c"); got != "chat_abc_msg_activity" {
		t.Fatalf("got %q", got)
	}
	if got := chatActivityDurableName(""); got != "chat_unknown_msg_activity" {
		t.Fatalf("empty got %q", got)
	}
}
