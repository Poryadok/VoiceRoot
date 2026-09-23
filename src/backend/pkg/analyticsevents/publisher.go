package analyticsevents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/pkg/analyticshash"
	"voice/backend/pkg/correlation"
	"voice/backend/pkg/natslog"
)

const streamName = "analytics_events"

var analyticsStreamConfig = nats.StreamConfig{
	Name:      streamName,
	Subjects:  []string{"analytics.>"},
	Retention: nats.LimitsPolicy,
	MaxAge:    7 * 24 * time.Hour,
	Storage:   nats.FileStorage,
}

// Publisher publishes analytics telemetry to analytics.* subjects.
type Publisher interface {
	Publish(ctx context.Context, subject, sourceService, eventType string, props map[string]any) error
}

// IdentityPublisher can hash account/profile IDs into analytics event fields (no PII in properties).
type IdentityPublisher interface {
	Publisher
	PublishWithAccount(ctx context.Context, subject, sourceService, eventType, accountID string, props map[string]any) error
}

// JetStreamPublisher implements Publisher via JetStream.
type JetStreamPublisher struct {
	nc      *nats.Conn
	js      nats.JetStreamContext
	Logger  *slog.Logger
	HashKey string
}

// NoopPublisher drops events.
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, string, string, string, map[string]any) error {
	return nil
}

func (NoopPublisher) PublishWithAccount(context.Context, string, string, string, string, map[string]any) error {
	return nil
}

func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-analytics-publisher"),
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
	info, err := js.StreamInfo(streamName)
	if err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("read deployment-owned analytics stream: %w", err)
	}
	if err := validateAnalyticsStream(info); err != nil {
		_ = nc.Drain()
		return nil, err
	}
	return &JetStreamPublisher{nc: nc, js: js}, nil
}

func (p *JetStreamPublisher) Close() {
	if p != nil && p.nc != nil {
		_ = p.nc.Drain()
	}
}

func validateAnalyticsStream(info *nats.StreamInfo) error {
	if info == nil || info.Config.Name != analyticsStreamConfig.Name ||
		!sameAnalyticsSubjects(info.Config.Subjects, analyticsStreamConfig.Subjects) ||
		info.Config.Retention != analyticsStreamConfig.Retention ||
		info.Config.MaxAge != analyticsStreamConfig.MaxAge ||
		info.Config.Storage != analyticsStreamConfig.Storage {
		return fmt.Errorf("deployment-owned analytics stream does not match required contract")
	}
	return nil
}

func sameAnalyticsSubjects(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i, subject := range want {
		if got[i] != subject {
			return false
		}
	}
	return true
}

func (p *JetStreamPublisher) Publish(ctx context.Context, subject, sourceService, eventType string, props map[string]any) error {
	return p.publishEvent(ctx, subject, sourceService, eventType, "", "", props)
}

func (p *JetStreamPublisher) PublishWithAccount(ctx context.Context, subject, sourceService, eventType, accountID string, props map[string]any) error {
	return p.publishEvent(ctx, subject, sourceService, eventType, accountID, "", props)
}

func (p *JetStreamPublisher) publishEvent(ctx context.Context, subject, sourceService, eventType, accountID, profileID string, props map[string]any) error {
	if p == nil || p.js == nil {
		return nil
	}
	propsJSON := "{}"
	if len(props) > 0 {
		if b, err := json.Marshal(props); err == nil {
			propsJSON = string(b)
		}
	}
	ev := &analyticsv1.AnalyticsEvent{
		EventId:        uuid.NewString(),
		EventType:      eventType,
		SourceService:  sourceService,
		Timestamp:      timestamppb.Now(),
		PropertiesJson: propsJSON,
	}
	if h := analyticshash.ID(p.HashKey, accountID); h != "" {
		ev.UserIdHashed = &h
	}
	if h := analyticshash.ID(p.HashKey, profileID); h != "" {
		ev.ProfileIdHashed = &h
	}
	data, err := protojson.Marshal(ev)
	if err != nil {
		return err
	}
	msg := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	requestID := correlation.FromGRPC(ctx)
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		natslog.LogPublishError(p.Logger, subject, requestID, err)
		return err
	}
	natslog.LogPublish(p.Logger, subject, requestID, "analytics event published")
	return nil
}
