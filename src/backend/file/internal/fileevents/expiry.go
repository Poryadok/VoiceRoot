package fileevents

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

func NewExpiryEvent(fileID string, chatID *string) (*eventsv1.FileStreamEvent, error) {
	if err := validateExpiryIDs(fileID, chatID); err != nil {
		return nil, err
	}
	env := newFileEvent()
	env.Payload = &eventsv1.FileStreamEvent_FileExpired{FileExpired: &eventsv1.FileExpired{FileId: fileID, ChatId: chatID}}
	return env, nil
}

func validateExpiryIDs(fileID string, chatID *string) error {
	invalid := errors.New("invalid expiry event authority")
	if id, err := uuid.Parse(fileID); err != nil || id == uuid.Nil {
		return invalid
	}
	if chatID != nil {
		if id, err := uuid.Parse(*chatID); err != nil || id == uuid.Nil {
			return invalid
		}
	}
	return nil
}

// PublishExpiryEvent sends the already committed envelope. A failed/ambiguous
// PubAck leaves the same event ID, timestamp and payload available for retry.
func (p *JetStreamPublisher) PublishExpiryEvent(ctx context.Context, payload []byte) error {
	var event eventsv1.FileStreamEvent
	if len(payload) > 8192 || proto.Unmarshal(payload, &event) != nil || event.GetFileExpired() == nil {
		return errors.New("invalid expiry outbox payload")
	}
	id, err := uuid.Parse(event.EventId)
	if err != nil || id == uuid.Nil || event.OccurredAt == nil || event.OccurredAt.CheckValid() != nil {
		return errors.New("invalid expiry event envelope")
	}
	if err = validateExpiryIDs(event.GetFileExpired().FileId, event.GetFileExpired().ChatId); err != nil {
		return err
	}
	return p.publishProto(ctx, subjectFileExpired, &event)
}
