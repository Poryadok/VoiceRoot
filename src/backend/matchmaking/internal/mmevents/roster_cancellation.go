package mmevents

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

// PublishRosterCancellation transmits the envelope saved by the reset transaction.
// It never invokes the compatibility producer that generates a fresh event ID.
func (p *JetStreamPublisher) PublishRosterCancellation(ctx context.Context, id uuid.UUID, payload []byte) error {
	env := new(eventsv1.MatchmakingStreamEvent)
	if p == nil || p.js == nil || id == uuid.Nil || proto.Unmarshal(payload, env) != nil || env.GetEventId() != id.String() || env.GetOccurredAt() == nil || env.GetOccurredAt().CheckValid() != nil {
		return fmt.Errorf("invalid roster cancellation envelope")
	}
	fact := env.GetSearchCancelled()
	if fact == nil || fact.GetReason() != "VOICE_ROSTER_CHANGED" {
		return fmt.Errorf("invalid roster cancellation payload")
	}
	session, sessionErr := uuid.Parse(fact.GetSessionId())
	profile, profileErr := uuid.Parse(fact.GetProfileId())
	if sessionErr != nil || profileErr != nil || session == uuid.Nil || profile == uuid.Nil {
		return fmt.Errorf("invalid roster cancellation identity")
	}
	message := &nats.Msg{Subject: subjectSearchCancel, Data: payload, Header: nats.Header{nats.MsgIdHdr: []string{id.String()}}}
	ack, err := p.js.PublishMsg(message, nats.Context(ctx))
	if err != nil {
		return err
	}
	if ack == nil {
		return fmt.Errorf("roster cancellation PubAck missing")
	}
	return nil
}
