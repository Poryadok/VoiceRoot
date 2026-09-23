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
	streamName                = "chat_events"
	subjectSpaceCreated       = "space.created"
	subjectSpaceTreeChanged   = "space.tree_changed"
	subjectVoiceRoomCreated   = "voice.room_created"
	subjectVoiceRoomDeleted   = "voice.room_deleted"
	subjectSpaceInviteCreated = "space.invite_created"
	subjectSpaceMemberJoined  = "space.member_joined"
	subjectSpaceMemberLeft    = "space.member_left"
	subjectSpaceUpdated       = "space.updated"
	subjectSpaceDeleted       = "space.deleted"
)

// JetStreamPublisher publishes ChatStreamEvent payloads to NATS JetStream.
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
		info, err := p.js.StreamInfo(streamName)
		if err != nil {
			p.ensureErr = fmt.Errorf("required JetStream stream %q is unavailable: %w", streamName, err)
			return
		}
		p.ensureErr = validateBootstrappedStream(info)
	})
	return p.ensureErr
}

// Validate verifies the centrally bootstrapped stream before serving traffic.
func (p *JetStreamPublisher) Validate() error { return p.ensureStream() }

func spaceEventStreamSubjects() []string {
	return []string{"chat.created", "chat.member_changed", "chat.dm_peer_deleted", subjectSpaceTreeChanged, subjectSpaceCreated, subjectVoiceRoomCreated, subjectVoiceRoomDeleted, subjectSpaceInviteCreated, subjectSpaceMemberJoined, subjectSpaceMemberLeft, subjectSpaceUpdated, subjectSpaceDeleted}
}

func streamHasSubject(info *nats.StreamInfo, subject string) bool {
	if info == nil {
		return false
	}
	for _, configured := range info.Config.Subjects {
		if configured == subject {
			return true
		}
	}
	return false
}

func validateBootstrappedStream(info *nats.StreamInfo) error {
	if info == nil || info.Config.Name != streamName || info.Config.Retention != nats.LimitsPolicy || info.Config.MaxAge != 7*24*time.Hour || info.Config.Storage != nats.FileStorage || len(info.Config.Subjects) != len(spaceEventStreamSubjects()) {
		return fmt.Errorf("required JetStream stream %q does not match bootstrap definition", streamName)
	}
	seen := make(map[string]struct{}, len(info.Config.Subjects))
	for _, subject := range info.Config.Subjects {
		seen[subject] = struct{}{}
	}
	for _, subject := range spaceEventStreamSubjects() {
		if _, ok := seen[subject]; !ok {
			return fmt.Errorf("required JetStream stream %q does not match bootstrap definition", streamName)
		}
	}
	return nil
}

// Publish sends a coordinator-prepared message without changing its subject,
// payload or headers and returns the JetStream acknowledgement for validation
// by the outbox delivery coordinator.
func (p *JetStreamPublisher) Publish(ctx context.Context, msg *nats.Msg) (*nats.PubAck, error) {
	if p == nil || p.js == nil {
		return nil, fmt.Errorf("jetstream publisher not initialized")
	}
	if ctx == nil {
		return nil, fmt.Errorf("jetstream publish context is nil")
	}
	if msg == nil {
		return nil, fmt.Errorf("jetstream prepared message is nil")
	}
	if err := p.ensureStream(); err != nil {
		return nil, err
	}
	ack, err := p.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return nil, fmt.Errorf("jetstream publish prepared %s: %w", msg.Subject, err)
	}
	return ack, nil
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
	msg.Header.Set(nats.MsgIdHdr, env.GetEventId())
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	if created := env.GetSpaceCreated(); created != nil {
		attrs = append(attrs, slog.String("space_id", created.GetSpaceId()), slog.String("owner_profile_id", created.GetOwnerProfileId()))
	}
	if invite := env.GetSpaceInviteCreated(); invite != nil {
		attrs = append(attrs, slog.String("space_id", invite.GetSpaceId()), slog.String("invite_code", invite.GetInviteCode()))
	}
	natslog.LogPublish(p.Logger, subject, requestID, "space event published", attrs...)
	return nil
}

// PublishTreeNodeUpserted implements Publisher.
func (p *JetStreamPublisher) PublishTreeNodeUpserted(ctx context.Context, spaceID, nodeID, kind, chatID, voiceRoomID string, isPinned bool, pinOrder *int32) error {
	changed := &eventsv1.SpaceTreeChanged{
		SpaceId:  spaceID,
		NodeId:   nodeID,
		Change:   "upserted",
		Kind:     kind,
		IsPinned: isPinned,
	}
	if chatID != "" {
		changed.ChatId = &chatID
	}
	if voiceRoomID != "" {
		changed.VoiceRoomId = &voiceRoomID
	}
	if pinOrder != nil {
		changed.PinOrder = pinOrder
	}
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceTreeChanged{
			SpaceTreeChanged: changed,
		},
	}
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

// PublishInviteCreated implements Publisher.
func (p *JetStreamPublisher) PublishInviteCreated(ctx context.Context, spaceID, inviteCode string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceInviteCreated{
			SpaceInviteCreated: &eventsv1.SpaceInviteCreated{
				SpaceId:    spaceID,
				InviteCode: inviteCode,
			},
		},
	}
	return p.publishProto(ctx, subjectSpaceInviteCreated, env)
}

// PublishSpaceCreated implements Publisher.
func (p *JetStreamPublisher) PublishSpaceCreated(ctx context.Context, spaceID, ownerProfileID string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceCreated{
			SpaceCreated: &eventsv1.SpaceCreated{
				SpaceId:        spaceID,
				OwnerProfileId: ownerProfileID,
			},
		},
	}
	return p.publishProto(ctx, subjectSpaceCreated, env)
}

func (p *JetStreamPublisher) PublishMemberJoined(ctx context.Context, spaceID, profileID string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceMemberJoined{
			SpaceMemberJoined: &eventsv1.SpaceMemberJoined{
				SpaceId:   spaceID,
				ProfileId: profileID,
			},
		},
	}
	return p.publishProto(ctx, subjectSpaceMemberJoined, env)
}

func (p *JetStreamPublisher) PublishMemberLeft(ctx context.Context, spaceID, profileID string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceMemberLeft{
			SpaceMemberLeft: &eventsv1.SpaceMemberLeft{
				SpaceId:   spaceID,
				ProfileId: profileID,
			},
		},
	}
	return p.publishProto(ctx, subjectSpaceMemberLeft, env)
}

func (p *JetStreamPublisher) PublishSpaceUpdated(ctx context.Context, spaceID string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceUpdated{
			SpaceUpdated: &eventsv1.SpaceUpdated{SpaceId: spaceID},
		},
	}
	return p.publishProto(ctx, subjectSpaceUpdated, env)
}

func (p *JetStreamPublisher) PublishSpaceDeleted(ctx context.Context, spaceID string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_SpaceDeleted{
			SpaceDeleted: &eventsv1.SpaceDeleted{SpaceId: spaceID},
		},
	}
	return p.publishProto(ctx, subjectSpaceDeleted, env)
}

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
