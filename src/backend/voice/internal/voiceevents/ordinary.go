package voiceevents

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/correlation"
	"voice/backend/pkg/natslog"
)

// PublishOrdinaryEvent accepts the original compatibility call envelope as well
// as BE-102's additive facts. It never rebuilds an envelope or regenerates IDs.
func (p *JetStreamPublisher) PublishOrdinaryEvent(ctx context.Context, id uuid.UUID, subject string, payload []byte) error {
	if subject == "voice.call_started" || subject == "voice.participant_joined" || subject == "voice.participant_left" {
		return p.PublishAdmissionEvent(ctx, id, subject, payload)
	}
	if p == nil || p.js == nil || id == uuid.Nil || len(payload) == 0 {
		return fmt.Errorf("invalid durable ordinary event")
	}
	env := new(eventsv1.VoiceStreamEvent)
	if err := proto.Unmarshal(payload, env); err != nil || env.EventId != id.String() || env.OccurredAt == nil || env.OccurredAt.CheckValid() != nil {
		return fmt.Errorf("invalid durable ordinary envelope")
	}
	valid := false
	switch subject {
	case "voice.call_incoming":
		valid = env.GetCallIncoming() != nil
	case "voice.call_accepted":
		valid = env.GetCallAccepted() != nil
	case "voice.call_declined":
		valid = env.GetCallDeclined() != nil
	case "voice.call_missed":
		valid = env.GetCallMissed() != nil
	case "voice.call_ended":
		valid = env.GetCallEnded() != nil
	}
	if !valid {
		return fmt.Errorf("ordinary subject does not match envelope")
	}
	msg := &nats.Msg{Subject: subject, Data: payload, Header: nats.Header{}}
	msg.Header.Set("Nats-Msg-Id", id.String())
	natslog.SetRequestIDHeader(msg.Header, correlation.FromGRPC(ctx))
	ack, err := p.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("publish durable ordinary event: %w", err)
	}
	if ack == nil {
		return fmt.Errorf("JetStream did not acknowledge ordinary event")
	}
	return nil
}
