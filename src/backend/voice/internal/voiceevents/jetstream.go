package voiceevents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

const streamName = "voice_events"

type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext

	ensureOnce sync.Once
	ensureErr  error
}

func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-service-voice-events"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	return &JetStreamPublisher{nc: nc, js: js}, nil
}

func (p *JetStreamPublisher) PublishCallIncoming(ctx context.Context, ev *eventsv1.CallIncoming) error {
	return p.publish(ctx, "voice.call_incoming", &eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload:    &eventsv1.VoiceStreamEvent_CallIncoming{CallIncoming: ev},
	})
}

func (p *JetStreamPublisher) PublishCallAccepted(ctx context.Context, ev *eventsv1.CallAccepted) error {
	return p.publish(ctx, "voice.call_accepted", &eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload:    &eventsv1.VoiceStreamEvent_CallAccepted{CallAccepted: ev},
	})
}

func (p *JetStreamPublisher) PublishCallDeclined(ctx context.Context, ev *eventsv1.CallDeclined) error {
	return p.publish(ctx, "voice.call_declined", &eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload:    &eventsv1.VoiceStreamEvent_CallDeclined{CallDeclined: ev},
	})
}

func (p *JetStreamPublisher) PublishCallMissed(ctx context.Context, ev *eventsv1.CallMissed) error {
	return p.publish(ctx, "voice.call_missed", &eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload:    &eventsv1.VoiceStreamEvent_CallMissed{CallMissed: ev},
	})
}

func (p *JetStreamPublisher) PublishCallEnded(ctx context.Context, ev *eventsv1.CallEnded) error {
	return p.publish(ctx, "voice.call_ended", &eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload:    &eventsv1.VoiceStreamEvent_CallEnded{CallEnded: ev},
	})
}

func (p *JetStreamPublisher) PublishVoiceStateChanged(ctx context.Context, ev *eventsv1.VoiceStateChanged) error {
	return p.publish(ctx, "voice.state_changed", &eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload:    &eventsv1.VoiceStreamEvent_VoiceStateChanged{VoiceStateChanged: ev},
	})
}

func (p *JetStreamPublisher) publish(_ context.Context, subject string, env *eventsv1.VoiceStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal VoiceStreamEvent: %w", err)
	}
	if _, err := p.js.Publish(subject, b); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	return nil
}

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		if _, err := p.js.StreamInfo(streamName); err == nil {
			return
		}
		_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
			Name:      streamName,
			Subjects:  []string{"voice.>"},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		})
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
