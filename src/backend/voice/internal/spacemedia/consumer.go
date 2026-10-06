package spacemedia

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/voice/internal/store"
)

const (
	spaceAccessStream  = "chat_events"
	spaceAccessDurable = "voice_space_media_chat"
	spaceAccessSubject = "space.voice_room_access_invalidated"
	spaceAccessTarget  = "_INBOX.voice.voice.space_media_chat"
	rolePolicyStream   = "role_events"
	rolePolicyDurable  = "voice_space_media_role"
	rolePolicySubject  = "role.voice_policy_invalidated"
	rolePolicyTarget   = "_INBOX.voice.voice.space_media_role"
)

type ackableInvalidation interface {
	AckSync(...nats.AckOpt) error
	NakWithDelay(time.Duration, ...nats.AckOpt) error
	Term(...nats.AckOpt) error
}

type InvalidationConsumer struct {
	nc   *nats.Conn
	subs []*nats.Subscription
}

func StartInvalidationConsumer(ctx context.Context, natsURL string, coordinator *Coordinator, logger *slog.Logger) (*InvalidationConsumer, error) {
	if ctx == nil || coordinator == nil || natsURL == "" {
		return nil, fmt.Errorf("Space media invalidation consumer is not configured")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-space-media-invalidations"),
		nats.CustomInboxPrefix("_INBOX.voice.voice"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect Voice media invalidation consumer: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("open Voice media invalidation JetStream: %w", err)
	}
	consumer := &InvalidationConsumer{nc: nc}
	bind := func(stream, durable, subject, target string, handler nats.MsgHandler) error {
		info, err := js.ConsumerInfo(stream, durable)
		if err != nil {
			return fmt.Errorf("read preprovisioned Voice media consumer %s/%s: %w", stream, durable, err)
		}
		config := info.Config
		if config.Durable != durable || config.FilterSubject != subject || config.DeliverSubject != target ||
			config.AckPolicy != nats.AckExplicitPolicy || config.DeliverPolicy != nats.DeliverNewPolicy {
			return fmt.Errorf("preprovisioned Voice media consumer %s/%s has incompatible contract", stream, durable)
		}
		sub, err := js.Subscribe(subject, handler, nats.Bind(stream, durable), nats.ManualAck())
		if err != nil {
			return fmt.Errorf("bind preprovisioned Voice media consumer %s/%s: %w", stream, durable, err)
		}
		consumer.subs = append(consumer.subs, sub)
		return nil
	}
	chatHandler := func(msg *nats.Msg) {
		if err := handleSpaceAccessInvalidation(ctx, msg.Subject, msg.Data, msg, coordinator); err != nil && logger != nil {
			logger.Warn("Space media access invalidation handling failed", slog.String("error", err.Error()))
		}
	}
	roleHandler := func(msg *nats.Msg) {
		if err := handleRolePolicyInvalidation(ctx, msg.Subject, msg.Data, msg, coordinator); err != nil && logger != nil {
			logger.Warn("Space media Role invalidation handling failed", slog.String("error", err.Error()))
		}
	}
	if err := bind(spaceAccessStream, spaceAccessDurable, spaceAccessSubject, spaceAccessTarget, chatHandler); err != nil {
		_ = consumer.Close()
		return nil, err
	}
	if err := bind(rolePolicyStream, rolePolicyDurable, rolePolicySubject, rolePolicyTarget, roleHandler); err != nil {
		_ = consumer.Close()
		return nil, err
	}
	return consumer, nil
}

func handleSpaceAccessInvalidation(ctx context.Context, subject string, data []byte, msg ackableInvalidation, coordinator *Coordinator) error {
	if msg == nil || coordinator == nil || subject != spaceAccessSubject {
		return terminalInvalidation(msg, fmt.Errorf("invalid Space access invalidation delivery"))
	}
	var envelope eventsv1.ChatStreamEvent
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return terminalInvalidation(msg, fmt.Errorf("decode Space access invalidation"))
	}
	access := envelope.GetVoiceRoomAccessInvalidated()
	if access == nil || !canonicalID(envelope.GetEventId()) || !canonicalID(access.GetSpaceId()) || access.GetAccessEpoch() == 0 ||
		!optionalCanonicalID(access.GetVoiceRoomId()) || !optionalCanonicalID(access.GetProfileId()) {
		return terminalInvalidation(msg, fmt.Errorf("invalid Space access invalidation envelope"))
	}
	if err := coordinator.Observe(ctx, AuthorityNotice{SpaceID: access.GetSpaceId(), Kind: store.SpaceAccessEpoch, Epoch: access.GetAccessEpoch(), VoiceRoomID: access.GetVoiceRoomId(), ProfileID: access.GetProfileId()}); err != nil {
		_ = msg.NakWithDelay(time.Second)
		return err
	}
	return msg.AckSync()
}

func handleRolePolicyInvalidation(ctx context.Context, subject string, data []byte, msg ackableInvalidation, coordinator *Coordinator) error {
	if msg == nil || coordinator == nil || subject != rolePolicySubject {
		return terminalInvalidation(msg, fmt.Errorf("invalid Role policy invalidation delivery"))
	}
	var envelope eventsv1.RoleStreamEvent
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return terminalInvalidation(msg, fmt.Errorf("decode Role policy invalidation"))
	}
	policy := envelope.GetVoiceRoomPolicyInvalidated()
	if policy == nil || !canonicalID(envelope.GetEventId()) || !canonicalID(policy.GetSpaceId()) || policy.GetPolicyEpoch() == 0 ||
		!optionalCanonicalID(policy.GetVoiceRoomId()) || !optionalCanonicalID(policy.GetProfileId()) {
		return terminalInvalidation(msg, fmt.Errorf("invalid Role policy invalidation envelope"))
	}
	if err := coordinator.Observe(ctx, AuthorityNotice{SpaceID: policy.GetSpaceId(), Kind: store.RolePolicyEpoch, Epoch: policy.GetPolicyEpoch(), VoiceRoomID: policy.GetVoiceRoomId(), ProfileID: policy.GetProfileId()}); err != nil {
		_ = msg.NakWithDelay(time.Second)
		return err
	}
	return msg.AckSync()
}

func terminalInvalidation(msg ackableInvalidation, cause error) error {
	if msg == nil {
		return cause
	}
	if err := msg.Term(); err != nil {
		return fmt.Errorf("%v; terminate invalidation: %w", cause, err)
	}
	return cause
}

func optionalCanonicalID(value string) bool { return value == "" || canonicalID(value) }

func (c *InvalidationConsumer) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.nc != nil {
		if err := c.nc.Drain(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
