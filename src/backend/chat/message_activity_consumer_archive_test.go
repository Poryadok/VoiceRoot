package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

type archivePreservingActivityStore struct {
	archived          bool
	touched           bool
	promoted          bool
	autoUnarchiveCall bool
}

func (s *archivePreservingActivityStore) TouchLastMessageAt(context.Context, uuid.UUID, time.Time) error {
	s.touched = true
	return nil
}

func (s *archivePreservingActivityStore) AutoUnarchiveDMRecipients(context.Context, uuid.UUID, uuid.UUID) error {
	s.autoUnarchiveCall = true
	s.archived = false
	return nil
}

func (s *archivePreservingActivityStore) PromoteDeclinedDMRecipients(context.Context, uuid.UUID, uuid.UUID) error {
	s.promoted = true
	return nil
}

func TestHandleMessageActivity_IncomingDMKeepsArchivedBadgeOnly(t *testing.T) {
	chatID := uuid.New()
	senderID := uuid.New()
	data, err := proto.Marshal(&eventsv1.MessageStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
			MessageId: uuid.NewString(), ChatId: chatID.String(), SenderProfileId: senderID.String(),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	store := &archivePreservingActivityStore{archived: true}
	handleMessageActivity(context.Background(), store, &nats.Msg{Data: data}, nil)

	if !store.touched {
		t.Fatal("incoming message must retain activity for the archive badge")
	}
	if !store.promoted {
		t.Fatal("incoming message must retain declined-DM re-contact handling")
	}
	if store.autoUnarchiveCall {
		t.Fatal("incoming message must not auto-unarchive an archived chat")
	}
	if !store.archived {
		t.Fatal("incoming DM must keep an archived chat archived")
	}
}
