package main

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

// TestMessageEventBytesToFanout_SentIncludesThreadParentID documents app stack0:
// message_create fan-out must expose thread_parent_id when MessageSent carries it.
func TestMessageEventBytesToFanout_SentIncludesThreadParentID(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	sender := uuid.NewString()
	parentID := uuid.NewString()

	sent := &eventsv1.MessageStreamEvent{
		EventId:    "e-thread",
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
	hdr := nats.Header{}
	hdr.Set(natsHeaderThreadParentID, parentID)

	gotChat, fe, ok := messageEventToFanout(b, hdr)
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
	if d["thread_parent_id"] != parentID {
		t.Fatalf("thread_parent_id=%v want %q", d["thread_parent_id"], parentID)
	}
}
