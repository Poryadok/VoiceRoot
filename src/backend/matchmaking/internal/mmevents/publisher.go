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
	streamName            = "matchmaking_events"
	subjectSearchStarted  = "mm.search_started"
	subjectSearchCancel   = "mm.search_cancelled"
	subjectMatchFound     = "mm.match_found"
	subjectMatchCompleted  = "mm.match_completed"
	subjectRatingSubmitted = "mm.rating_submitted"
	subjectSearchNudge     = "mm.search_nudge"
	subjectSearchTimeout   = "mm.search_timeout"
)

// MatchFoundEvent is emitted when a match proposal is created.
type MatchFoundEvent struct {
	MatchID    string
	GameID     string
	Mode       string
	Region     string
	ProfileIDs []string
	SessionIDs []string
}

// MatchCompletedEvent is emitted when all participants left a match squad.
type MatchCompletedEvent struct {
	MatchID         string
	DurationSeconds int64
	ProfileIDs      []string
}

// RatingSubmittedEvent is emitted when a peer rating is stored.
type RatingSubmittedEvent struct {
	MatchID        string
	RaterProfileID string
	RatedProfileID string
	Stars          int32
}

// Publisher publishes matchmaking domain events.
type Publisher interface {
	PublishSearchStarted(ctx context.Context, sessionID, profileID, gameID, mode, region string) error
	PublishSearchCancelled(ctx context.Context, sessionID, profileID string) error
	PublishMatchFound(ctx context.Context, ev MatchFoundEvent) error
	PublishMatchCompleted(ctx context.Context, ev MatchCompletedEvent) error
	PublishRatingSubmitted(ctx context.Context, ev RatingSubmittedEvent) error
	PublishSearchNudge(ctx context.Context, sessionID, profileID, gameID, mode string) error
	PublishSearchTimeout(ctx context.Context, sessionID, profileID, gameID, mode string) error
	Close() error
}

// NoopPublisher drops events (tests / NATS optional).
type NoopPublisher struct{}

func (NoopPublisher) PublishSearchStarted(context.Context, string, string, string, string, string) error {
	return nil
}
func (NoopPublisher) PublishSearchCancelled(context.Context, string, string) error { return nil }
func (NoopPublisher) PublishMatchFound(context.Context, MatchFoundEvent) error      { return nil }
func (NoopPublisher) PublishMatchCompleted(context.Context, MatchCompletedEvent) error { return nil }
func (NoopPublisher) PublishRatingSubmitted(context.Context, RatingSubmittedEvent) error { return nil }
func (NoopPublisher) PublishSearchNudge(context.Context, string, string, string, string) error {
	return nil
}
func (NoopPublisher) PublishSearchTimeout(context.Context, string, string, string, string) error {
	return nil
}
func (NoopPublisher) Close() error { return nil }

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
				subjectSearchNudge,
				subjectSearchTimeout,
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
	env := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MatchmakingStreamEvent_SearchStarted{
			SearchStarted: &eventsv1.SearchStarted{
				SessionId: sessionID,
				ProfileId: profileID,
				GameId:    gameID,
				Mode:      mode,
				Region:    region,
			},
		},
	}
	return p.publishProto(ctx, subjectSearchStarted, env)
}

// PublishMatchFound implements Publisher.
func (p *JetStreamPublisher) PublishMatchFound(ctx context.Context, ev MatchFoundEvent) error {
	env := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MatchmakingStreamEvent_MatchFound{
			MatchFound: &eventsv1.MatchFound{
				MatchId:    ev.MatchID,
				ProfileIds: ev.ProfileIDs,
				GameId:     ev.GameID,
				Mode:       ev.Mode,
				Region:     ev.Region,
				SessionIds: ev.SessionIDs,
			},
		},
	}
	return p.publishProto(ctx, subjectMatchFound, env)
}

// PublishMatchCompleted implements Publisher.
func (p *JetStreamPublisher) PublishMatchCompleted(ctx context.Context, ev MatchCompletedEvent) error {
	env := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MatchmakingStreamEvent_MatchCompleted{
			MatchCompleted: &eventsv1.MatchCompleted{
				MatchId:         ev.MatchID,
				DurationSeconds: ev.DurationSeconds,
				ProfileIds:      ev.ProfileIDs,
			},
		},
	}
	return p.publishProto(ctx, subjectMatchCompleted, env)
}

// PublishRatingSubmitted implements Publisher.
func (p *JetStreamPublisher) PublishRatingSubmitted(ctx context.Context, ev RatingSubmittedEvent) error {
	env := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MatchmakingStreamEvent_RatingSubmitted{
			RatingSubmitted: &eventsv1.RatingSubmitted{
				MatchId:         ev.MatchID,
				RaterProfileId:  ev.RaterProfileID,
				RatedProfileId:  ev.RatedProfileID,
				Stars:           ev.Stars,
			},
		},
	}
	return p.publishProto(ctx, subjectRatingSubmitted, env)
}

// PublishSearchNudge implements Publisher.
func (p *JetStreamPublisher) PublishSearchNudge(ctx context.Context, sessionID, profileID, gameID, mode string) error {
	env := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MatchmakingStreamEvent_SearchNudge{
			SearchNudge: &eventsv1.SearchNudge{
				SessionId: sessionID,
				ProfileId: profileID,
				GameId:    gameID,
				Mode:      mode,
			},
		},
	}
	return p.publishProto(ctx, subjectSearchNudge, env)
}

// PublishSearchTimeout implements Publisher.
func (p *JetStreamPublisher) PublishSearchTimeout(ctx context.Context, sessionID, profileID, gameID, mode string) error {
	env := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MatchmakingStreamEvent_MatchTimeout{
			MatchTimeout: &eventsv1.MatchTimeout{
				SessionId: sessionID,
				ProfileId: profileID,
				GameId:    gameID,
				Mode:      mode,
			},
		},
	}
	return p.publishProto(ctx, subjectSearchTimeout, env)
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
