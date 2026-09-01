package main

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
	messageEventsStreamName   = "message_events"
	subjectMessageDeliveryAck = "message.delivery_ack"
)

// deliveryAckPublisher publishes durable delivery acknowledgements to message.events.
type deliveryAckPublisher interface {
	PublishDeliveryAck(ctx context.Context, chatID, messageID, recipientProfileID string) error
}

type jetstreamDeliveryAckPublisher struct {
	js         nats.JetStreamContext
	ensureOnce sync.Once
	ensureErr  error
}

func newJetstreamDeliveryAckPublisher(nc *nats.Conn) (*jetstreamDeliveryAckPublisher, error) {
	if nc == nil {
		return nil, fmt.Errorf("nil nats connection")
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	return &jetstreamDeliveryAckPublisher{js: js}, nil
}

func (p *jetstreamDeliveryAckPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("delivery ack publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		info, err := p.js.StreamInfo(messageEventsStreamName)
		if err != nil {
			_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
				Name:      messageEventsStreamName,
				Subjects:  []string{"message.>"},
				Retention: nats.LimitsPolicy,
				MaxAge:    7 * 24 * time.Hour,
				Storage:   nats.FileStorage,
			})
			return
		}
		for _, subject := range info.Config.Subjects {
			if subject == subjectMessageDeliveryAck || subject == "message.>" {
				return
			}
		}
		cfg := info.Config
		cfg.Subjects = append(append([]string(nil), info.Config.Subjects...), subjectMessageDeliveryAck)
		_, p.ensureErr = p.js.UpdateStream(&cfg)
	})
	return p.ensureErr
}

func (p *jetstreamDeliveryAckPublisher) PublishDeliveryAck(ctx context.Context, chatID, messageID, recipientProfileID string) error {
	_ = ctx
	if err := p.ensureStream(); err != nil {
		return err
	}
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_DeliveryAck{
			DeliveryAck: &eventsv1.DeliveryAck{
				MessageId: messageID,
				ChatId:    chatID,
				ProfileId: recipientProfileID,
			},
		},
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal delivery ack: %w", err)
	}
	if _, err := p.js.Publish(subjectMessageDeliveryAck, b); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subjectMessageDeliveryAck, err)
	}
	return nil
}
