package messageevents

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
	streamName            = "message_events"
	subjectMessageSent    = "message.sent"
	subjectMessageEdited  = "message.edited"
	subjectMessageDeleted = "message.deleted"
	subjectMessageRead       = "message.read"
	subjectReactionAdded     = "message.reaction_added"
	subjectReactionRemoved   = "message.reaction_removed"
	subjectMentionAdded      = "message.mention_added"
	subjectMessagePinned     = "message.pinned"
	subjectMessageUnpinned   = "message.unpinned"
	natsHeaderThreadParentID = "X-Voice-Thread-Parent-Id"
)

// JetStreamPublisher publishes MessageStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	// Logger emits structured nats_publish lines; optional.
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS_URL, prepares JetStream handle, and lazily ensures stream message_events
// (logical domain message.events per CONTRACT_MATRIX; JetStream resource names cannot contain '.' in nats.go).
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-messaging-message-events"),
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

// EnsureStream creates the message_events JetStream stream if it does not exist yet.
func (p *JetStreamPublisher) EnsureStream() error {
	return p.ensureStream()
}

func messageEventStreamSubjects() []string {
	return []string{
		subjectMessageSent,
		subjectMessageEdited,
		subjectMessageDeleted,
		subjectMessageRead,
		subjectReactionAdded,
		subjectReactionRemoved,
		subjectMentionAdded,
		subjectMessagePinned,
		subjectMessageUnpinned,
	}
}

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		desired := messageEventStreamSubjects()
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

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.MessageStreamEvent) error {
	return p.publishProtoWithHeaders(ctx, subject, env, nil)
}

func (p *JetStreamPublisher) publishProtoWithHeaders(ctx context.Context, subject string, env *eventsv1.MessageStreamEvent, extra nats.Header) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal MessageStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	for k, vals := range extra {
		for _, v := range vals {
			msg.Header.Add(k, v)
		}
	}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "message event published", messageEventLogAttrs(env)...)
	return nil
}

func messageEventLogAttrs(env *eventsv1.MessageStreamEvent) []slog.Attr {
	if env == nil {
		return nil
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	switch p := env.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		if s := p.MessageSent; s != nil {
			attrs = append(attrs, slog.String("message_id", s.GetMessageId()), slog.String("chat_id", s.GetChatId()))
		}
	case *eventsv1.MessageStreamEvent_MessageEdited:
		if e := p.MessageEdited; e != nil {
			attrs = append(attrs, slog.String("message_id", e.GetMessageId()), slog.String("chat_id", e.GetChatId()))
		}
	case *eventsv1.MessageStreamEvent_MessageDeleted:
		if d := p.MessageDeleted; d != nil {
			attrs = append(attrs, slog.String("message_id", d.GetMessageId()), slog.String("chat_id", d.GetChatId()))
		}
	case *eventsv1.MessageStreamEvent_MessageRead:
		if r := p.MessageRead; r != nil {
			attrs = append(attrs, slog.String("message_id", r.GetMessageId()), slog.String("chat_id", r.GetChatId()), slog.String("profile_id", r.GetProfileId()))
		}
	case *eventsv1.MessageStreamEvent_ReactionAdded:
		if r := p.ReactionAdded; r != nil {
			attrs = append(attrs, slog.String("message_id", r.GetMessageId()), slog.String("chat_id", r.GetChatId()), slog.String("profile_id", r.GetProfileId()))
		}
	case *eventsv1.MessageStreamEvent_ReactionRemoved:
		if r := p.ReactionRemoved; r != nil {
			attrs = append(attrs, slog.String("message_id", r.GetMessageId()), slog.String("chat_id", r.GetChatId()), slog.String("profile_id", r.GetProfileId()))
		}
	case *eventsv1.MessageStreamEvent_MentionAdded:
		if m := p.MentionAdded; m != nil {
			attrs = append(attrs, slog.String("message_id", m.GetMessageId()), slog.String("chat_id", m.GetChatId()))
		}
	case *eventsv1.MessageStreamEvent_MessagePinned:
		if m := p.MessagePinned; m != nil {
			attrs = append(attrs, slog.String("message_id", m.GetMessageId()), slog.String("chat_id", m.GetChatId()))
		}
	case *eventsv1.MessageStreamEvent_MessageUnpinned:
		if m := p.MessageUnpinned; m != nil {
			attrs = append(attrs, slog.String("message_id", m.GetMessageId()), slog.String("chat_id", m.GetChatId()))
		}
	}
	return attrs
}

// PublishMessageSent implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMessageSent(ctx context.Context, messageID, chatID, senderProfileID string, hasMentions bool, threadParentID string, isE2E bool) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       messageID,
				ChatId:          chatID,
				SenderProfileId: senderProfileID,
				HasMentions:     hasMentions,
				ThreadParentId:  ptrIfNonEmpty(threadParentID),
				IsE2E:           isE2E,
			},
		},
	}
	return p.publishProtoWithHeaders(ctx, subjectMessageSent, env, messageSentPublishHeaders(threadParentID))
}

func messageSentPublishHeaders(threadParentID string) nats.Header {
	if strings.TrimSpace(threadParentID) == "" {
		return nil
	}
	h := nats.Header{}
	h.Set(natsHeaderThreadParentID, strings.TrimSpace(threadParentID))
	return h
}

func ptrIfNonEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := strings.TrimSpace(s)
	return &v
}

// PublishMentionAdded implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMentionAdded(ctx context.Context, messageID, chatID, senderProfileID string, mentionedProfileIDs []string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MentionAdded{
			MentionAdded: &eventsv1.MentionAdded{
				MessageId:           messageID,
				ChatId:              chatID,
				SenderProfileId:     senderProfileID,
				MentionedProfileIds: append([]string(nil), mentionedProfileIDs...),
			},
		},
	}
	return p.publishProto(ctx, subjectMentionAdded, env)
}

// PublishMessageEdited implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMessageEdited(ctx context.Context, messageID, chatID string, isE2E bool) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{
				MessageId: messageID,
				ChatId:    chatID,
				IsE2E:     isE2E,
			},
		},
	}
	return p.publishProto(ctx, subjectMessageEdited, env)
}

// PublishMessageRead implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMessageRead(ctx context.Context, messageID, chatID, profileID string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MessageRead{
			MessageRead: &eventsv1.MessageRead{
				MessageId: messageID,
				ChatId:    chatID,
				ProfileId: profileID,
			},
		},
	}
	return p.publishProto(ctx, subjectMessageRead, env)
}

// PublishMessageDeleted implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMessageDeleted(ctx context.Context, messageID, chatID string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MessageDeleted{
			MessageDeleted: &eventsv1.MessageDeleted{
				MessageId: messageID,
				ChatId:    chatID,
			},
		},
	}
	return p.publishProto(ctx, subjectMessageDeleted, env)
}

// PublishReactionAdded implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishReactionAdded(ctx context.Context, messageID, chatID, profileID, messageAuthorProfileID, emoji string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId:              messageID,
				ChatId:                 chatID,
				ProfileId:              profileID,
				Emoji:                  emoji,
				MessageAuthorProfileId: messageAuthorProfileID,
			},
		},
	}
	return p.publishProto(ctx, subjectReactionAdded, env)
}

// PublishReactionRemoved implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishReactionRemoved(ctx context.Context, messageID, chatID, profileID, emoji string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_ReactionRemoved{
			ReactionRemoved: &eventsv1.ReactionRemoved{
				MessageId: messageID,
				ChatId:    chatID,
				ProfileId: profileID,
				Emoji:     emoji,
			},
		},
	}
	return p.publishProto(ctx, subjectReactionRemoved, env)
}

// PublishMessagePinned implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMessagePinned(ctx context.Context, messageID, chatID, pinnedBy string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MessagePinned{
			MessagePinned: &eventsv1.MessagePinned{
				MessageId: messageID,
				ChatId:    chatID,
				PinnedBy:  pinnedBy,
			},
		},
	}
	return p.publishProto(ctx, subjectMessagePinned, env)
}

// PublishMessageUnpinned implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMessageUnpinned(ctx context.Context, messageID, chatID, unpinnedBy string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MessageUnpinned{
			MessageUnpinned: &eventsv1.MessageUnpinned{
				MessageId:  messageID,
				ChatId:     chatID,
				UnpinnedBy: unpinnedBy,
			},
		},
	}
	return p.publishProto(ctx, subjectMessageUnpinned, env)
}

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
