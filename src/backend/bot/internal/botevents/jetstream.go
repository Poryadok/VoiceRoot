package botevents

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
	streamName           = "bot_events"
	subjectBotRegistered = "bot.registered"
	subjectCommandExec   = "bot.command_executed"
	subjectWebhookDeliv  = "bot.webhook_delivered"
)

// JetStreamPublisher publishes BotStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS_URL and prepares JetStream.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-bot-events"),
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

func (p *JetStreamPublisher) Close() {
	if p != nil && p.nc != nil {
		_ = p.nc.Drain()
	}
}

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		subjects := []string{subjectBotRegistered, subjectCommandExec, subjectWebhookDeliv}
		if info, err := p.js.StreamInfo(streamName); err == nil {
			for _, subj := range subjects {
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
			Name:      streamName,
			Subjects:  subjects,
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
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

func (p *JetStreamPublisher) publish(ctx context.Context, subject string, evt *eventsv1.BotStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	evt.EventId = uuid.NewString()
	if evt.OccurredAt == nil {
		evt.OccurredAt = timestamppb.Now()
	}
	raw, err := proto.Marshal(evt)
	if err != nil {
		return err
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: raw, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "bot event published", slog.String("event_id", evt.GetEventId()))
	return nil
}

// PublishBotRegistered emits bot.registered.
func (p *JetStreamPublisher) PublishBotRegistered(ctx context.Context, botID, ownerID string) error {
	return p.publish(ctx, subjectBotRegistered, &eventsv1.BotStreamEvent{
		Payload: &eventsv1.BotStreamEvent_BotRegistered{
			BotRegistered: &eventsv1.BotRegistered{
				BotId:          botID,
				OwnerAccountId: ownerID,
			},
		},
	})
}

// PublishCommandExecuted emits bot.command_executed.
func (p *JetStreamPublisher) PublishCommandExecuted(ctx context.Context, botID, command, chatID string) error {
	return p.publish(ctx, subjectCommandExec, &eventsv1.BotStreamEvent{
		Payload: &eventsv1.BotStreamEvent_CommandExecuted{
			CommandExecuted: &eventsv1.CommandExecuted{
				BotId:   botID,
				Command: command,
				ChatId:  chatID,
			},
		},
	})
}

// PublishWebhookDelivered emits bot.webhook_delivered.
func (p *JetStreamPublisher) PublishWebhookDelivered(ctx context.Context, botID, deliveryID string, success bool) error {
	return p.publish(ctx, subjectWebhookDeliv, &eventsv1.BotStreamEvent{
		Payload: &eventsv1.BotStreamEvent_WebhookDelivered{
			WebhookDelivered: &eventsv1.WebhookDelivered{
				BotId:      botID,
				DeliveryId: deliveryID,
				Success:    success,
			},
		},
	})
}
