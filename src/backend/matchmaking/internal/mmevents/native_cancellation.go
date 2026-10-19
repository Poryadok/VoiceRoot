package mmevents

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

func (p *JetStreamPublisher) PublishCapturedCancellation(ctx context.Context, id uuid.UUID, payload []byte) error {
	envelope := new(eventsv1.MatchmakingStreamEvent)
	if p == nil || p.js == nil || ctx == nil || ctx.Err() != nil || id == uuid.Nil || len(payload) == 0 || len(payload) > 16384 || proto.Unmarshal(payload, envelope) != nil || envelope.EventId != id.String() || envelope.OccurredAt == nil || envelope.OccurredAt.CheckValid() != nil || len(envelope.ProtoReflect().GetUnknown()) != 0 || len(envelope.OccurredAt.ProtoReflect().GetUnknown()) != 0 {
		return fmt.Errorf("invalid captured cancellation envelope")
	}
	fact := envelope.GetSearchCancelled()
	if fact == nil || (fact.Reason != "" && fact.Reason != "VOICE_ROSTER_CHANGED") || len(fact.ProtoReflect().GetUnknown()) != 0 {
		return fmt.Errorf("invalid captured cancellation fact")
	}
	session, e1 := uuid.Parse(fact.SessionId)
	profile, e2 := uuid.Parse(fact.ProfileId)
	if e1 != nil || e2 != nil || session == uuid.Nil || profile == uuid.Nil || session.String() != fact.SessionId || profile.String() != fact.ProfileId {
		return fmt.Errorf("invalid captured cancellation identity")
	}
	canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
	if err != nil || !bytes.Equal(canonical, payload) {
		return fmt.Errorf("noncanonical captured cancellation envelope")
	}
	ack, err := p.js.PublishMsg(&nats.Msg{Subject: subjectSearchCancel, Data: canonical, Header: nats.Header{nats.MsgIdHdr: []string{id.String()}}}, nats.Context(ctx))
	if err != nil {
		return err
	}
	if ack == nil {
		return fmt.Errorf("captured cancellation PubAck missing")
	}
	return nil
}
