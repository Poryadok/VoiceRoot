package moderationevents

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
	streamName             = "moderation_events"
	subjectReportCreated   = "moderation.report_created"
	subjectSanctionApplied = "moderation.sanction_applied"
	subjectAppealSubmitted = "moderation.appeal_submitted"
)

// Publisher publishes moderation.events domain payloads.
type Publisher interface {
	PublishReportCreated(ctx context.Context, reportID, reporterProfileID string) error
	PublishSanctionApplied(ctx context.Context, sanctionID, targetAccountID, sanctionType string) error
	PublishAppealSubmitted(ctx context.Context, appealID, sanctionID string) error
	Close() error
}

// JetStreamPublisher publishes ModerationStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc     *nats.Conn
	js     jetStreamClient
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}
type jetStreamClient interface {
	StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error)
	PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error)
}

// NewJetStreamPublisher connects to NATS and prepares JetStream for moderation.events.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-moderation-events"),
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

func moderationStreamSubjects() []string {
	return []string{
		subjectReportCreated,
		subjectSanctionApplied,
		subjectAppealSubmitted,
	}
}

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		info, err := p.js.StreamInfo(streamName)
		if err != nil {
			p.ensureErr = fmt.Errorf("required JetStream stream %q is unavailable: %w", streamName, err)
			return
		}
		p.ensureErr = validateBootstrappedStream(info)
	})
	return p.ensureErr
}
func validateBootstrappedStream(info *nats.StreamInfo) error {
	expected := moderationStreamSubjects()
	if info == nil || info.Config.Name != streamName || info.Config.Retention != nats.LimitsPolicy || info.Config.MaxAge != 7*24*time.Hour || info.Config.Storage != nats.FileStorage || len(info.Config.Subjects) != len(expected) {
		return fmt.Errorf("required JetStream stream %q does not match bootstrap definition", streamName)
	}
	seen := map[string]struct{}{}
	for _, s := range info.Config.Subjects {
		seen[s] = struct{}{}
	}
	for _, s := range expected {
		if _, ok := seen[s]; !ok {
			return fmt.Errorf("required JetStream stream %q does not match bootstrap definition", streamName)
		}
	}
	return nil
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.ModerationStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal ModerationStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "moderation event published",
		slog.String("event_id", env.GetEventId()),
	)
	return nil
}

func newModerationEvent() *eventsv1.ModerationStreamEvent {
	return &eventsv1.ModerationStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
	}
}

func (p *JetStreamPublisher) PublishReportCreated(ctx context.Context, reportID, reporterProfileID string) error {
	env := newModerationEvent()
	env.Payload = &eventsv1.ModerationStreamEvent_ReportCreated{
		ReportCreated: &eventsv1.ReportCreated{
			ReportId:          reportID,
			ReporterProfileId: reporterProfileID,
		},
	}
	return p.publishProto(ctx, subjectReportCreated, env)
}

func (p *JetStreamPublisher) PublishSanctionApplied(ctx context.Context, sanctionID, targetAccountID, sanctionType string) error {
	env := newModerationEvent()
	env.Payload = &eventsv1.ModerationStreamEvent_SanctionApplied{
		SanctionApplied: &eventsv1.SanctionApplied{
			SanctionId:      sanctionID,
			TargetAccountId: targetAccountID,
			Type:            sanctionType,
		},
	}
	return p.publishProto(ctx, subjectSanctionApplied, env)
}

func (p *JetStreamPublisher) PublishAppealSubmitted(ctx context.Context, appealID, sanctionID string) error {
	env := newModerationEvent()
	env.Payload = &eventsv1.ModerationStreamEvent_AppealSubmitted{
		AppealSubmitted: &eventsv1.AppealSubmitted{
			AppealId:   appealID,
			SanctionId: sanctionID,
		},
	}
	return p.publishProto(ctx, subjectAppealSubmitted, env)
}

// Close drains the NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
