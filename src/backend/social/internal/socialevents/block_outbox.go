package socialevents

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

// NewBlockOutboxEvent creates the existing user_blocked envelope once, inside
// the block transaction. Both wire bytes and deduplication ID survive retries.
func NewBlockOutboxEvent(blocker, blocked uuid.UUID) (uuid.UUID, []byte, error) {
	env := newSocialEvent()
	env.Payload = &eventsv1.SocialStreamEvent_UserBlocked{UserBlocked: &eventsv1.UserBlocked{
		BlockerAccountId: blocker.String(), BlockedAccountId: blocked.String(),
	}}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(env)
	return uuid.MustParse(env.GetEventId()), payload, err
}

// PublishBlockOutboxEvent returns success only for a positive broker PubAck.
// Consumer application remains independently idempotent and at-least-once.
func (p *JetStreamPublisher) PublishBlockOutboxEvent(ctx context.Context, eventID uuid.UUID, payload []byte) error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	var env eventsv1.SocialStreamEvent
	if err := proto.Unmarshal(payload, &env); err != nil || eventID == uuid.Nil || env.GetEventId() != eventID.String() || env.GetUserBlocked() == nil {
		return fmt.Errorf("invalid block outbox event")
	}
	msg := &nats.Msg{Subject: subjectUserBlocked, Data: payload, Header: nats.Header{}}
	msg.Header.Set(nats.MsgIdHdr, eventID.String())
	ack, err := p.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subjectUserBlocked, err)
	}
	if ack == nil || ack.Sequence == 0 || ack.Stream != streamName {
		return fmt.Errorf("block event missing positive social_events PubAck")
	}
	return nil
}
