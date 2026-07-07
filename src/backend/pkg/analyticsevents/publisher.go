package analyticsevents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/pkg/correlation"
	"voice/backend/pkg/natslog"
)

const streamName = "analytics_events"

// Publisher publishes analytics telemetry to analytics.* subjects.
type Publisher interface {
	Publish(ctx context.Context, subject, sourceService, eventType string, props map[string]any) error
}

// JetStreamPublisher implements Publisher via JetStream.
type JetStreamPublisher struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NoopPublisher drops events.
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, string, string, string, map[string]any) error {
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
	return &JetStreamPublisher{nc: nc, js: js}, nil
}

func (p *JetStreamPublisher) Close() {
	if p != nil && p.nc != nil {
		_ = p.nc.Drain()
	}
}

func (p *JetStreamPublisher) EnsureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("analytics publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		if _, err := p.js.StreamInfo(streamName); err == nil {
			return
		}
		_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
			Name:      streamName,
			Subjects:  []string{"analytics.>"},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
		})
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) Publish(ctx context.Context, subject, sourceService, eventType string, props map[string]any) error {
	if p == nil || p.js == nil {
		return nil
	}
	if err := p.EnsureStream(); err != nil {
		return err
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
	data, err := protojson.Marshal(ev)
	if err != nil {
		return err
	}
	msg := &nats.Msg{Subject: subject, Data: data}
	if rid := correlation.FromGRPC(ctx); rid != "" {
		natslog.SetRequestIDHeader(msg, rid)
	}
	if _, err := p.js.PublishMsg(msg); err != nil {
		natslog.LogPublishError(p.Logger, msg, err)
		return err
	}
	natslog.LogPublish(p.Logger, msg)
	return nil
}
