package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/natslog"
)

const (
	friendAcceptedStream  = "social_events"
	friendAcceptedDurable = "chat_friend_accepted"
	friendAcceptedSubject = "social.friend_accepted"
)

type friendDMRequestStore interface {
	PromoteFriendDMRequests(ctx context.Context, profileA, profileB uuid.UUID) error
}

type acceptedFriendChecker interface {
	AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
}

func friendAcceptedConsumerConfig() *nats.ConsumerConfig {
	return &nats.ConsumerConfig{
		Durable:        friendAcceptedDurable,
		DeliverSubject: "_INBOX.voice.chat." + friendAcceptedDurable,
		DeliverGroup:   friendAcceptedDurable,
		DeliverPolicy:  nats.DeliverAllPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
		FilterSubject:  friendAcceptedSubject,
	}
}

func validateFriendAcceptedDurable(js nats.JetStreamContext) error {
	want := friendAcceptedConsumerConfig()
	info, err := js.ConsumerInfo(friendAcceptedStream, want.Durable)
	if err != nil {
		return fmt.Errorf("inspect friend accepted durable: %w", err)
	}
	if info == nil || info.Stream != friendAcceptedStream || info.Name != want.Durable ||
		info.Config.Durable != want.Durable || info.Config.DeliverSubject != want.DeliverSubject ||
		info.Config.DeliverGroup != want.DeliverGroup || info.Config.DeliverPolicy != want.DeliverPolicy ||
		info.Config.AckPolicy != want.AckPolicy || info.Config.FilterSubject != want.FilterSubject ||
		len(info.Config.FilterSubjects) != 0 {
		return fmt.Errorf("friend accepted durable %q has incompatible configuration", want.Durable)
	}
	return nil
}

func handleFriendAccepted(ctx context.Context, store friendDMRequestStore, friends acceptedFriendChecker, msg *nats.Msg, logger *slog.Logger) error {
	if msg == nil || msg.Subject != friendAcceptedSubject {
		return nil
	}
	var env eventsv1.SocialStreamEvent
	if err := proto.Unmarshal(msg.Data, &env); err != nil || env.GetFriendAdded() == nil {
		natslog.LogConsume(logger, msg, slog.LevelWarn, "invalid friend accepted payload")
		return nil
	}
	a, errA := uuid.Parse(env.GetFriendAdded().GetRequesterProfileId())
	b, errB := uuid.Parse(env.GetFriendAdded().GetTargetProfileId())
	if errA != nil || errB != nil || a == b {
		natslog.LogConsume(logger, msg, slog.LevelWarn, "invalid friend accepted profile pair")
		return nil
	}
	if friends == nil {
		return fmt.Errorf("friend status unavailable")
	}
	accepted, err := friends.AreFriends(ctx, a, b)
	if err != nil {
		return err
	}
	if !accepted {
		return nil
	}
	if err := store.PromoteFriendDMRequests(ctx, a, b); err != nil {
		natslog.LogConsume(logger, msg, slog.LevelWarn, "friend DM request promotion failed", slog.String("error", err.Error()))
		return err
	}
	natslog.LogConsume(logger, msg, slog.LevelInfo, "friend DM requests promoted")
	return nil
}

func subscribeFriendAccepted(ctx context.Context, js nats.JetStreamContext, store friendDMRequestStore, friends acceptedFriendChecker, logger *slog.Logger) (*nats.Subscription, error) {
	if store == nil || friends == nil {
		return nil, fmt.Errorf("friend DM dependencies not configured")
	}
	if err := validateFriendAcceptedDurable(js); err != nil {
		return nil, err
	}
	return js.QueueSubscribe(friendAcceptedSubject, friendAcceptedDurable, func(msg *nats.Msg) {
		if err := handleFriendAccepted(ctx, store, friends, msg, logger); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	}, nats.Bind(friendAcceptedStream, friendAcceptedDurable), nats.ManualAck())
}

func runFriendAcceptedConsumer(ctx context.Context, natsURL string, store friendDMRequestStore, friends acceptedFriendChecker, logger *slog.Logger) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("friend accepted consumer: missing NATS URL")
	}
	for {
		err := runFriendAcceptedConsumerOnce(ctx, natsURL, store, friends, logger)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if logger != nil {
			logger.Warn("friend accepted consumer retrying", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func runFriendAcceptedConsumerOnce(ctx context.Context, natsURL string, store friendDMRequestStore, friends acceptedFriendChecker, logger *slog.Logger) error {
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-chat-friend-accepted"),
		nats.CustomInboxPrefix("_INBOX.voice.chat"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer func() { _ = nc.Drain() }()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}
	sub, err := subscribeFriendAccepted(ctx, js, store, friends, logger)
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	<-ctx.Done()
	return ctx.Err()
}
