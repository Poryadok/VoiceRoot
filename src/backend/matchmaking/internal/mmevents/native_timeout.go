package mmevents

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"strings"
	eventsv1 "voice.app/voice/events/v1"
)

func (p *JetStreamPublisher) PublishCapturedTimeout(ctx context.Context, id uuid.UUID, payload []byte) error {
	envelope := new(eventsv1.MatchmakingStreamEvent)
	if p == nil || p.js == nil || ctx == nil || ctx.Err() != nil || id == uuid.Nil || len(payload) == 0 || len(payload) > 16384 || proto.Unmarshal(payload, envelope) != nil || envelope.EventId != id.String() || envelope.OccurredAt == nil || envelope.OccurredAt.CheckValid() != nil || len(envelope.ProtoReflect().GetUnknown()) != 0 || len(envelope.OccurredAt.ProtoReflect().GetUnknown()) != 0 {
		return fmt.Errorf("invalid captured timeout envelope")
	}
	fact := envelope.GetMatchTimeout()
	if fact == nil || fact.Mode == "" || strings.TrimSpace(fact.Mode) != fact.Mode || len(fact.ProtoReflect().GetUnknown()) != 0 {
		return fmt.Errorf("invalid captured timeout fact")
	}
	for _, text := range []string{fact.SessionId, fact.ProfileId, fact.GameId} {
		value, err := uuid.Parse(text)
		if err != nil || value == uuid.Nil || value.String() != text {
			return fmt.Errorf("invalid captured timeout identity")
		}
	}
	canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
	if err != nil || !bytes.Equal(canonical, payload) {
		return fmt.Errorf("noncanonical captured timeout envelope")
	}
	ack, err := p.js.PublishMsg(&nats.Msg{Subject: subjectSearchTimeout, Data: canonical, Header: nats.Header{nats.MsgIdHdr: []string{id.String()}}}, nats.Context(ctx))
	if err != nil {
		return err
	}
	if ack == nil {
		return fmt.Errorf("captured timeout PubAck missing")
	}
	return nil
}
