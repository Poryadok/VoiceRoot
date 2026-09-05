package chatevents

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
	streamName               = "chat_events"
	subjectChatCreated       = "chat.created"
	subjectChatMemberChanged = "chat.member_changed"
	subjectDMPeerDeleted     = "chat.dm_peer_deleted"
)

// JetStreamPublisher publishes ChatStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	// Logger emits structured nats_publish lines; optional.
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS_URL, prepares JetStream handle, and lazily ensures stream chat_events
// (logical domain chat.events per CONTRACT_MATRIX; JetStream resource names cannot contain '.' in nats.go).
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-chat-chat-events"),
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
		if err == nil {
			for _, subject := range info.Config.Subjects {
				if subject == subjectDMPeerDeleted {
					return
				}
			}
			config := info.Config
			config.Subjects = append(config.Subjects, subjectDMPeerDeleted)
			_, p.ensureErr = p.js.UpdateStream(&config)
			return
		}
		_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
			Name: streamName,
			Subjects: []string{
				subjectChatCreated,
				subjectChatMemberChanged,
				subjectDMPeerDeleted,
			},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		})
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.ChatStreamEvent) error {
	return p.publishProtoWithMsgID(ctx, subject, env, "")
}

func (p *JetStreamPublisher) publishProtoWithMsgID(ctx context.Context, subject string, env *eventsv1.ChatStreamEvent, msgID string) error {
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
	if msgID != "" {
		msg.Header.Set(nats.MsgIdHdr, msgID)
	}
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	if created := env.GetChatCreated(); created != nil {
		attrs = append(attrs, slog.String("chat_id", created.GetChatId()))
	}
	if changed := env.GetChatMemberChanged(); changed != nil {
		attrs = append(attrs, slog.String("chat_id", changed.GetChatId()), slog.String("profile_id", changed.GetProfileId()))
	}
	if deleted := env.GetDmPeerDeleted(); deleted != nil {
		attrs = append(attrs, slog.String("chat_id", deleted.GetChatId()), slog.String("recipient_profile_id", deleted.GetRecipientProfileId()))
	}
	natslog.LogPublish(p.Logger, subject, requestID, "chat event published", attrs...)
	return nil
}

// PublishChatCreated implements Publisher.
func (p *JetStreamPublisher) PublishChatCreated(ctx context.Context, chatID, chatType string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_ChatCreated{
			ChatCreated: &eventsv1.ChatCreated{
				ChatId: chatID,
				Type:   chatType,
			},
		},
	}
	return p.publishProto(ctx, subjectChatCreated, env)
}

// PublishChatMemberChanged implements Publisher.
func (p *JetStreamPublisher) PublishChatMemberChanged(ctx context.Context, chatID, profileID, change string) error {
	env := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{
			ChatMemberChanged: &eventsv1.ChatMemberChanged{
				ChatId:    chatID,
				ProfileId: profileID,
				Change:    change,
			},
		},
	}
	return p.publishProto(ctx, subjectChatMemberChanged, env)
}

// PublishDMPeerDeleted emits a targeted account-deletion acceleration event.
// eventID is supplied by the consumer and is also the JetStream de-duplication key.
func (p *JetStreamPublisher) PublishDMPeerDeleted(ctx context.Context, eventID string, chatID, recipientProfileID uuid.UUID) error {
	if eventID == "" {
		return fmt.Errorf("empty dm peer-deleted event id")
	}
	if chatID == uuid.Nil || recipientProfileID == uuid.Nil {
		return fmt.Errorf("invalid dm peer-deleted target")
	}
	env := &eventsv1.ChatStreamEvent{
		EventId:    eventID,
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.ChatStreamEvent_DmPeerDeleted{
			DmPeerDeleted: &eventsv1.DmPeerDeleted{
				ChatId:             chatID.String(),
				RecipientProfileId: recipientProfileID.String(),
			},
		},
	}
	return p.publishProtoWithMsgID(ctx, subjectDMPeerDeleted, env, eventID)
}

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
