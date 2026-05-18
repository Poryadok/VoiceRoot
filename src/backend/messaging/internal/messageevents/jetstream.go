package messageevents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

const (
	streamName            = "message_events"
	subjectMessageSent    = "message.sent"
	subjectMessageEdited = "message.edited"
	subjectMessageDeleted = "message.deleted"
)

// JetStreamPublisher publishes MessageStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext

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
				subjectMessageSent,
				subjectMessageEdited,
				subjectMessageDeleted,
				"message.reaction_added",
			},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		})
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.MessageStreamEvent) error {
	_ = ctx
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal MessageStreamEvent: %w", err)
	}
	if _, err := p.js.Publish(subject, b); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	return nil
}

// PublishMessageSent implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMessageSent(ctx context.Context, messageID, chatID, senderProfileID string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:         messageID,
				ChatId:            chatID,
				SenderProfileId:   senderProfileID,
			},
		},
	}
	return p.publishProto(ctx, subjectMessageSent, env)
}

// PublishMessageEdited implements MessageEventsPublisher.
func (p *JetStreamPublisher) PublishMessageEdited(ctx context.Context, messageID, chatID string) error {
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{
				MessageId: messageID,
				ChatId:    chatID,
			},
		},
	}
	return p.publishProto(ctx, subjectMessageEdited, env)
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

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
