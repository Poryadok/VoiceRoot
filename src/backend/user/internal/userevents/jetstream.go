package userevents

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/correlation"
	"voice/backend/pkg/natslog"
)

const (
	streamName                = "user_events"
	subjectProfileCreated     = "user.profile_created"
	subjectProfileUpdated     = "user.profile_updated"
	subjectProfileSwitched    = "user.profile_switched"
	subjectProfileVerified    = "user.verified"
)

// JetStreamPublisher publishes UserStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS_URL and prepares JetStream for user.events.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-user-user-events"),
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

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		if _, err := p.js.StreamInfo(streamName); err == nil {
			return
		}
		_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
			Name: streamName,
			Subjects: []string{
				subjectProfileCreated,
				subjectProfileUpdated,
				subjectProfileSwitched,
				subjectProfileVerified,
			},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		})
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.UserStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal UserStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "user event published",
		slog.String("event_id", env.GetEventId()))
	return nil
}

// PublishProfileCreated emits user.profile_created.
func (p *JetStreamPublisher) PublishProfileCreated(ctx context.Context, profileID, accountID string) error {
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_ProfileCreated{
			ProfileCreated: &eventsv1.ProfileCreated{
				ProfileId: profileID,
				AccountId: accountID,
			},
		},
	}
	return p.publishProto(ctx, subjectProfileCreated, env)
}

// PublishProfileUpdated emits user.profile_updated using the profile_created arm
// so search indexers can upsert without requiring regenerated protobuf stubs.
func (p *JetStreamPublisher) PublishProfileUpdated(ctx context.Context, profileID, accountID, _ string) error {
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_ProfileCreated{
			ProfileCreated: &eventsv1.ProfileCreated{
				ProfileId: profileID,
				AccountId: accountID,
			},
		},
	}
	return p.publishProto(ctx, subjectProfileUpdated, env)
}

// PublishProfileSwitched emits user.profile_switched.
func (p *JetStreamPublisher) PublishProfileSwitched(ctx context.Context, accountID, oldProfileID, newProfileID string) error {
	_ = oldProfileID
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_ProfileSwitched{
			ProfileSwitched: &eventsv1.ProfileSwitched{
				AccountId: accountID,
				ProfileId: newProfileID,
			},
		},
	}
	return p.publishProto(ctx, subjectProfileSwitched, env)
}

// PublishVerified emits user.verified.
func (p *JetStreamPublisher) PublishVerified(ctx context.Context, profileID, accountID, verificationType string) error {
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_ProfileCreated{
			ProfileCreated: &eventsv1.ProfileCreated{
				ProfileId: profileID,
				AccountId: accountID,
			},
		},
	}
	_ = verificationType
	return p.publishProto(ctx, subjectProfileVerified, env)
}

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
