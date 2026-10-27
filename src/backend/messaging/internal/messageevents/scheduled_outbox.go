package messageevents

import (
	"errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
	eventsv1 "voice.app/voice/events/v1"
)

// WithScheduledMetadata preserves the event identity and all ordinary delivery
// fields; the returned bytes are persisted once with the scheduled transition.
func WithScheduledMetadata(event OutboxEvent, at *time.Time) (OutboxEvent, error) {
	var envelope eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(event.Payload, &envelope); err != nil {
		return OutboxEvent{}, err
	}
	sent := envelope.GetMessageSent()
	if sent == nil {
		return OutboxEvent{}, errors.New("scheduled dispatch requires message.sent")
	}
	sent.WasScheduled = true
	if at != nil {
		sent.ScheduledAt = timestamppb.New(at.UTC())
	} else {
		sent.ScheduledAt = nil
	}
	return encodeOutboxEvent(event.Subject, &envelope, event.Headers)
}
