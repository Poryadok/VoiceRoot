package storyevents

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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
	streamName                  = "story_events"
	subjectStoryCreated         = "story.created"
	subjectStoryViewed          = "story.viewed"
	subjectStoryReacted         = "story.reacted"
	subjectStoryExpired         = "story.expired"
	subjectStoryHighlightCreated = "story.highlight_created"
	subjectStoryLfpCreated      = "story.lfp_created"
)

// JetStreamPublisher publishes StoryStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	// Logger emits structured nats_publish lines; optional.
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS_URL, prepares JetStream handle, and lazily ensures stream story_events
// (logical domain story.events per CONTRACT_MATRIX; JetStream resource names cannot contain '.' in nats.go).
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-story-story-events"),
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
				subjectStoryCreated,
				subjectStoryViewed,
				subjectStoryReacted,
				subjectStoryExpired,
				subjectStoryHighlightCreated,
				subjectStoryLfpCreated,
			},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		})
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.StoryStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal StoryStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	if created := env.GetStoryCreated(); created != nil {
		attrs = append(attrs, slog.String("story_id", created.GetStoryId()))
	}
	if viewed := env.GetStoryViewed(); viewed != nil {
		attrs = append(attrs, slog.String("story_id", viewed.GetStoryId()))
	}
	natslog.LogPublish(p.Logger, subject, requestID, "story event published", attrs...)
	return nil
}

func newStoryEvent() *eventsv1.StoryStreamEvent {
	return &eventsv1.StoryStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
	}
}

// PublishStoryCreated implements Publisher.
func (p *JetStreamPublisher) PublishStoryCreated(ctx context.Context, storyID, authorProfileID, storyType, gameTag string, mentionProfileIDs []string) error {
	created := &eventsv1.StoryCreated{
		StoryId:           storyID,
		AuthorProfileId:   authorProfileID,
		Type:              storyType,
		MentionProfileIds: append([]string(nil), mentionProfileIDs...),
	}
	if strings.TrimSpace(gameTag) != "" {
		created.GameTag = &gameTag
	}
	env := newStoryEvent()
	env.Payload = &eventsv1.StoryStreamEvent_StoryCreated{StoryCreated: created}
	return p.publishProto(ctx, subjectStoryCreated, env)
}

// PublishStoryViewed implements Publisher.
func (p *JetStreamPublisher) PublishStoryViewed(ctx context.Context, storyID, viewerProfileID string) error {
	env := newStoryEvent()
	env.Payload = &eventsv1.StoryStreamEvent_StoryViewed{
		StoryViewed: &eventsv1.StoryViewed{
			StoryId:         storyID,
			ViewerProfileId: viewerProfileID,
		},
	}
	return p.publishProto(ctx, subjectStoryViewed, env)
}

// PublishStoryReacted implements Publisher.
func (p *JetStreamPublisher) PublishStoryReacted(ctx context.Context, storyID, reactorProfileID, emoji string) error {
	env := newStoryEvent()
	env.Payload = &eventsv1.StoryStreamEvent_StoryReacted{
		StoryReacted: &eventsv1.StoryReacted{
			StoryId:          storyID,
			ReactorProfileId: reactorProfileID,
			Emoji:            emoji,
		},
	}
	return p.publishProto(ctx, subjectStoryReacted, env)
}

// PublishStoryExpired implements Publisher.
func (p *JetStreamPublisher) PublishStoryExpired(ctx context.Context, storyID string) error {
	env := newStoryEvent()
	env.Payload = &eventsv1.StoryStreamEvent_StoryExpired{
		StoryExpired: &eventsv1.StoryExpired{StoryId: storyID},
	}
	return p.publishProto(ctx, subjectStoryExpired, env)
}

// PublishStoryHighlightCreated implements Publisher.
func (p *JetStreamPublisher) PublishStoryHighlightCreated(ctx context.Context, highlightID, profileID string) error {
	env := newStoryEvent()
	env.Payload = &eventsv1.StoryStreamEvent_StoryHighlightCreated{
		StoryHighlightCreated: &eventsv1.StoryHighlightCreated{
			HighlightId: highlightID,
			ProfileId:   profileID,
		},
	}
	return p.publishProto(ctx, subjectStoryHighlightCreated, env)
}

// PublishStoryLfpCreated implements Publisher.
func (p *JetStreamPublisher) PublishStoryLfpCreated(ctx context.Context, storyID, authorProfileID, criteriaJSON string) error {
	env := newStoryEvent()
	env.Payload = &eventsv1.StoryStreamEvent_StoryLfpCreated{
		StoryLfpCreated: &eventsv1.StoryLfpCreated{
			StoryId:         storyID,
			AuthorProfileId: authorProfileID,
			CriteriaJson:    criteriaJSON,
		},
	}
	return p.publishProto(ctx, subjectStoryLfpCreated, env)
}

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
