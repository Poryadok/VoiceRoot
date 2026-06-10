package spaceevents

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
	streamName              = "chat_events"
	subjectSpaceCreated     = "space.created"
	subjectSpaceTreeChanged = "space.tree_changed"
	subjectVoiceRoomCreated = "space.voice_room_created"
	subjectVoiceRoomDeleted = "space.voice_room_deleted"
)

// JetStreamPublisher publishes ChatStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS_URL, prepares JetStream handle, and lazily ensures stream chat_events.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-space-space-events"),
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
		if info, err := p.js.StreamInfo(streamName); err == nil {
			for _, subj := range []string{subjectSpaceCreated, subjectSpaceTreeChanged, subjectVoiceRoomCreated, subjectVoiceRoomDeleted} {
				if !streamHasSubject(info, subj) {
					cfg := info.Config
					cfg.Subjects = append(cfg.Subjects, subj)
					_, p.ensureErr = p.js.UpdateStream(&cfg)
					if p.ensureErr != nil {
						return
					}
					info, p.ensureErr = p.js.StreamInfo(streamName)
					if p.ensureErr != nil {
						return
					}
				}
			}
			return
		}
		_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
			Name: streamName,
			Subjects: []string{
				"chat.created",
				"chat.member_changed",
				subjectSpaceTreeChanged,
				subjectSpaceCreated,
				subjectVoiceRoomCreated,
				subjectVoiceRoomDeleted,
			},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		})
	})
	return p.ensureErr
}

func streamHasSubject(info *nats.StreamInfo, subject string) bool {
	if info == nil {
		return false
	}
	for _, s := range info.Config.Subjects {
		if s == subject {
			return true
		}
	}
	return false
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.ChatStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal ChatStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	if created := env.GetSpaceCreated(); created != nil {
		attrs = append(attrs, slog.String("space_id", created.GetSpaceId()), slog.String("owner_profile_id", created.GetOwnerProfileId()))
	}
	natslog.LogPublish(p.Logger, subject, requestID, "space event published", attrs...)
	return nil
}

// PublishTreeNodeUpserted implements Publisher.
func (p *JetStreamPublisher) PublishTreeNodeUpserted(ctx context.Context, spaceID, nodeID, kind, chatID, voiceRoomID string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceTreeChanged{
			SpaceTreeChanged: &eventsv1.SpaceTreeChanged{
				SpaceId: spaceID,
				NodeId:  nodeID,
				Change:  "upserted",
			},
		},
	}
	_ = kind
	_ = chatID
	_ = voiceRoomID
	return p.publishProto(ctx, subjectSpaceTreeChanged, env)
}

// PublishTreeNodeRemoved implements Publisher.
func (p *JetStreamPublisher) PublishTreeNodeRemoved(ctx context.Context, spaceID, nodeID string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceTreeChanged{
			SpaceTreeChanged: &eventsv1.SpaceTreeChanged{
				SpaceId: spaceID,
				NodeId:  nodeID,
				Change:  "removed",
			},
		},
	}
	return p.publishProto(ctx, subjectSpaceTreeChanged, env)
}

// PublishVoiceRoomCreated implements Publisher.
func (p *JetStreamPublisher) PublishVoiceRoomCreated(ctx context.Context, spaceID, voiceRoomID string) error {
	_ = spaceID
	_ = voiceRoomID
	return nil // subject reserved for future VoiceRoomCreated payload
}

// PublishVoiceRoomDeleted implements Publisher.
func (p *JetStreamPublisher) PublishVoiceRoomDeleted(ctx context.Context, spaceID, voiceRoomID string) error {
	_ = spaceID
	_ = voiceRoomID
	return nil
}

// PublishSpaceCreated implements Publisher.
func (p *JetStreamPublisher) PublishSpaceCreated(ctx context.Context, spaceID, ownerProfileID string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceCreated{
			SpaceCreated: &eventsv1.SpaceCreated{
				SpaceId:         spaceID,
				OwnerProfileId: ownerProfileID,
			},
		},
	}
	return p.publishProto(ctx, subjectSpaceCreated, env)
}

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
