package socialevents

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
	streamName            = "social_events"
	subjectFriendRequest  = "social.friend_request"
	subjectFriendAccepted = "social.friend_accepted"
	subjectFriendRemoved  = "social.friend_removed"
	subjectUserBlocked    = "social.user_blocked"
	subjectContactsSynced = "social.contacts_synced"
)

// Publisher publishes social.events domain payloads.
type Publisher interface {
	PublishFriendRequest(ctx context.Context, requestID, requesterProfileID, targetProfileID string) error
	PublishFriendAccepted(ctx context.Context, requesterProfileID, targetProfileID string) error
	PublishFriendRemoved(ctx context.Context, profileA, profileB string) error
	PublishUserBlocked(ctx context.Context, blockerAccountID, blockedAccountID string) error
	PublishContactsSynced(ctx context.Context, ownerProfileID string, count int32) error
	Close() error
}

// JetStreamPublisher publishes SocialStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS and prepares JetStream for social.events.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-social-events"),
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

func socialStreamSubjects() []string {
	return []string{
		subjectFriendRequest,
		subjectFriendAccepted,
		subjectFriendRemoved,
		subjectUserBlocked,
		subjectContactsSynced,
	}
}

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		desired := socialStreamSubjects()
		info, err := p.js.StreamInfo(streamName)
		if err != nil {
			_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
				Name:      streamName,
				Subjects:  desired,
				Retention: nats.LimitsPolicy,
				MaxAge:    7 * 24 * time.Hour,
				Storage:   nats.FileStorage,
			})
			return
		}
		existing := make(map[string]struct{}, len(info.Config.Subjects))
		for _, subject := range info.Config.Subjects {
			existing[subject] = struct{}{}
		}
		merged := append([]string(nil), info.Config.Subjects...)
		for _, subject := range desired {
			if _, ok := existing[subject]; ok {
				continue
			}
			merged = append(merged, subject)
		}
		if len(merged) == len(info.Config.Subjects) {
			return
		}
		cfg := info.Config
		cfg.Subjects = merged
		_, p.ensureErr = p.js.UpdateStream(&cfg)
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.SocialStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal SocialStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "social event published",
		slog.String("event_id", env.GetEventId()),
	)
	return nil
}

func newSocialEvent() *eventsv1.SocialStreamEvent {
	return &eventsv1.SocialStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
	}
}

func (p *JetStreamPublisher) PublishFriendRequest(ctx context.Context, requestID, requesterProfileID, targetProfileID string) error {
	if requestID == "" {
		requestID = uuid.NewString()
	}
	env := newSocialEvent()
	env.Payload = &eventsv1.SocialStreamEvent_FriendRequest{
		FriendRequest: &eventsv1.FriendRequest{
			RequestId:           requestID,
			RequesterProfileId: requesterProfileID,
			TargetProfileId:    targetProfileID,
		},
	}
	return p.publishProto(ctx, subjectFriendRequest, env)
}

func (p *JetStreamPublisher) PublishFriendAccepted(ctx context.Context, requesterProfileID, targetProfileID string) error {
	env := newSocialEvent()
	env.Payload = &eventsv1.SocialStreamEvent_FriendAdded{
		FriendAdded: &eventsv1.FriendAdded{
			RequesterProfileId: requesterProfileID,
			TargetProfileId:    targetProfileID,
		},
	}
	return p.publishProto(ctx, subjectFriendAccepted, env)
}

func (p *JetStreamPublisher) PublishFriendRemoved(ctx context.Context, profileA, profileB string) error {
	env := newSocialEvent()
	env.Payload = &eventsv1.SocialStreamEvent_FriendRemoved{
		FriendRemoved: &eventsv1.FriendRemoved{
			ProfileIdA: profileA,
			ProfileIdB: profileB,
		},
	}
	return p.publishProto(ctx, subjectFriendRemoved, env)
}

func (p *JetStreamPublisher) PublishUserBlocked(ctx context.Context, blockerAccountID, blockedAccountID string) error {
	env := newSocialEvent()
	env.Payload = &eventsv1.SocialStreamEvent_UserBlocked{
		UserBlocked: &eventsv1.UserBlocked{
			BlockerAccountId: blockerAccountID,
			BlockedAccountId: blockedAccountID,
		},
	}
	return p.publishProto(ctx, subjectUserBlocked, env)
}

func (p *JetStreamPublisher) PublishContactsSynced(ctx context.Context, ownerProfileID string, count int32) error {
	env := newSocialEvent()
	env.Payload = &eventsv1.SocialStreamEvent_ContactSynced{
		ContactSynced: &eventsv1.ContactSynced{
			OwnerProfileId: ownerProfileID,
			Count:          count,
		},
	}
	return p.publishProto(ctx, subjectContactsSynced, env)
}

// Close drains the NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
