package main

import (
	"context"
	"fmt"
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
	deliveryAckStreamName  = "message_events"
	deliveryAckSubject     = "message.delivery_ack"
)

type deliveryAckPublisher interface {
	PublishDeliveryAck(ctx context.Context, chatID, messageID, recipientProfileID string) error
}

type jetStreamDeliveryAckPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext

	ensureOnce sync.Once
	ensureErr  error
}

func newDeliveryAckPublisher(natsURL string) (*jetStreamDeliveryAckPublisher, error) {
	if strings.TrimSpace(natsURL) == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-realtime-delivery-ack"),
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
	return &jetStreamDeliveryAckPublisher{nc: nc, js: js}, nil
}

func (p *jetStreamDeliveryAckPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		desired := []string{
			"message.sent", "message.edited", "message.deleted", "message.read",
			"message.reaction_added", "message.reaction_removed", "message.mention_added",
			"message.pinned", "message.unpinned", "message.forwarded", deliveryAckSubject,
		}
		info, err := p.js.StreamInfo(deliveryAckStreamName)
		if err != nil {
			_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
				Name:      deliveryAckStreamName,
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

func (p *jetStreamDeliveryAckPublisher) PublishDeliveryAck(ctx context.Context, chatID, messageID, recipientProfileID string) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.MessageStreamEvent_DeliveryAck{
			DeliveryAck: &eventsv1.MessageDeliveryAck{
				MessageId: strings.TrimSpace(messageID),
				ChatId:    strings.TrimSpace(chatID),
				ProfileId: strings.TrimSpace(recipientProfileID),
			},
		},
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal delivery_ack: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: deliveryAckSubject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", deliveryAckSubject, err)
	}
	return nil
}

func (p *jetStreamDeliveryAckPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
