package mmevents

import (
	"context"
	"encoding/json"
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
	streamName           = "matchmaking_events"
	subjectSearchStarted = "mm.search_started"
	subjectSearchCancel  = "mm.search_cancelled"
)

// Publisher publishes matchmaking domain events.
type Publisher interface {
	PublishSearchStarted(ctx context.Context, sessionID, profileID, gameID, mode, region string) error
	PublishSearchCancelled(ctx context.Context, sessionID, profileID string) error
	Close() error
}

// NoopPublisher drops events (tests / NATS optional).
type NoopPublisher struct{}

func (NoopPublisher) PublishSearchStarted(context.Context, string, string, string, string, string) error {
	return nil
}
func (NoopPublisher) PublishSearchCancelled(context.Context, string, string) error { return nil }
func (NoopPublisher) Close() error                                               { return nil }

// searchCancelledJSON is published until generated SearchCancelled proto is wired (buf generate).
type searchCancelledJSON struct {
	EventID   string `json:"event_id"`
	SessionID string `json:"session_id"`
	ProfileID string `json:"profile_id"`
}

// JetStreamPublisher publishes to matchmaking.events stream.
type JetStreamPublisher struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS and prepares JetStream.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-matchmaking-events"),
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
				subjectSearchStarted,
				subjectSearchCancel,
				"mm.search_timeout",
				"mm.match_found",
				"mm.match_completed",
				"mm.rating_submitted",
				"mm.player_banned",
			},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		})
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.MatchmakingStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal MatchmakingStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "matchmaking event published",
		slog.String("event_id", env.GetEventId()))
	return nil
}

func (p *JetStreamPublisher) publishJSON(ctx context.Context, subject string, payload any) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "matchmaking event published")
	return nil
}

// PublishSearchStarted implements Publisher.
func (p *JetStreamPublisher) PublishSearchStarted(ctx context.Context, sessionID, profileID, gameID, mode, region string) error {
	_ = mode
	_ = region // mode/region in proto after buf generate (jetstream_events.proto fields 4–5)
	env := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MatchmakingStreamEvent_SearchStarted{
			SearchStarted: &eventsv1.SearchStarted{
				SessionId: sessionID,
				ProfileId: profileID,
				GameId:    gameID,
			},
		},
	}
	return p.publishProto(ctx, subjectSearchStarted, env)
}

// PublishSearchCancelled implements Publisher.
func (p *JetStreamPublisher) PublishSearchCancelled(ctx context.Context, sessionID, profileID string) error {
	return p.publishJSON(ctx, subjectSearchCancel, searchCancelledJSON{
		EventID:   uuid.NewString(),
		SessionID: sessionID,
		ProfileID: profileID,
	})
}

// Close drains the NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
