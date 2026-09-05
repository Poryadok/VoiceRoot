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
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/natslog"
)

const (
	userEventsStreamName             = "user_events"
	userAccountDeletedSubject        = "user.account_deleted"
	dmPeerDeletedChildEventIDVersion = "voice.chat.dm_peer_deleted.v1"
)

type accountDeletedProfileLister interface {
	ListProfileIDsForAccount(context.Context, string) ([]uuid.UUID, error)
}

type dmPeerDeletionTargetStore interface {
	ListDMPeerDeletionTargets(context.Context, []uuid.UUID) ([]store.DMPeerDeletionTarget, error)
}

type dmPeerDeletedPublisher interface {
	PublishDMPeerDeleted(context.Context, string, uuid.UUID, uuid.UUID) error
}

type accountDeletedDMConsumer struct {
	profiles  accountDeletedProfileLister
	targets   dmPeerDeletionTargetStore
	publisher dmPeerDeletedPublisher
	logger    *slog.Logger
}

func newAccountDeletedDMConsumer(
	profiles accountDeletedProfileLister,
	targets dmPeerDeletionTargetStore,
	publisher dmPeerDeletedPublisher,
	logger *slog.Logger,
) *accountDeletedDMConsumer {
	return &accountDeletedDMConsumer{
		profiles:  profiles,
		targets:   targets,
		publisher: publisher,
		logger:    logger,
	}
}

func requiredEventUUID(raw, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid user.account_deleted %s", field)
	}
	return id, nil
}

// dmPeerDeletedChildEventID is a UUIDv5 derived solely from the source event
// and the canonical fanout target. It is therefore stable across redelivery.
func dmPeerDeletedChildEventID(sourceEventID, chatID, survivingProfileID uuid.UUID) string {
	name := strings.Join([]string{
		dmPeerDeletedChildEventIDVersion,
		sourceEventID.String(),
		chatID.String(),
		survivingProfileID.String(),
	}, ":")
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(name)).String()
}

func (c *accountDeletedDMConsumer) handleUserAccountDeleted(ctx context.Context, msg *nats.Msg) error {
	if c == nil || c.profiles == nil || c.targets == nil || c.publisher == nil {
		return fmt.Errorf("user.account_deleted consumer not configured")
	}
	if msg == nil {
		return fmt.Errorf("user.account_deleted message is nil")
	}

	var env eventsv1.UserStreamEvent
	if err := proto.Unmarshal(msg.Data, &env); err != nil {
		return fmt.Errorf("unmarshal user.account_deleted: %w", err)
	}
	sourceEventID, err := requiredEventUUID(env.GetEventId(), "event_id")
	if err != nil {
		return err
	}
	payload := env.GetUserAccountDeleted()
	if payload == nil {
		return fmt.Errorf("user.account_deleted payload is required")
	}
	accountID, err := requiredEventUUID(payload.GetAccountId(), "account_id")
	if err != nil {
		return err
	}

	deletedProfileIDs, err := c.profiles.ListProfileIDsForAccount(ctx, accountID.String())
	if err != nil {
		return fmt.Errorf("list account profiles: %w", err)
	}
	if len(deletedProfileIDs) == 0 {
		return nil
	}
	for _, profileID := range deletedProfileIDs {
		if profileID == uuid.Nil {
			return fmt.Errorf("user service returned invalid profile_id")
		}
	}

	targets, err := c.targets.ListDMPeerDeletionTargets(ctx, deletedProfileIDs)
	if err != nil {
		return fmt.Errorf("list DM peer-deletion targets: %w", err)
	}
	for _, target := range targets {
		if target.ChatID == uuid.Nil || target.SurvivingProfileID == uuid.Nil {
			return fmt.Errorf("store returned invalid DM peer-deletion target")
		}
		childEventID := dmPeerDeletedChildEventID(sourceEventID, target.ChatID, target.SurvivingProfileID)
		if err := c.publisher.PublishDMPeerDeleted(ctx, childEventID, target.ChatID, target.SurvivingProfileID); err != nil {
			return fmt.Errorf("publish chat.dm_peer_deleted: %w", err)
		}
	}
	return nil
}

func accountDeletedDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "chat_" + strings.ReplaceAll(id, "-", "") + "_account_deleted"
}

func subscribeAccountDeletedConsumer(
	js nats.JetStreamContext,
	profiles accountDeletedProfileLister,
	targets dmPeerDeletionTargetStore,
	publisher dmPeerDeletedPublisher,
	instanceID string,
	logger *slog.Logger,
) (*nats.Subscription, error) {
	return subscribeAccountDeletedConsumerWithContext(context.Background(), js, profiles, targets, publisher, instanceID, logger)
}

func subscribeAccountDeletedConsumerWithContext(
	ctx context.Context,
	js nats.JetStreamContext,
	profiles accountDeletedProfileLister,
	targets dmPeerDeletionTargetStore,
	publisher dmPeerDeletedPublisher,
	instanceID string,
	logger *slog.Logger,
) (*nats.Subscription, error) {
	if js == nil {
		return nil, fmt.Errorf("user.account_deleted consumer JetStream not configured")
	}
	consumer := newAccountDeletedDMConsumer(profiles, targets, publisher, logger)
	handler := func(msg *nats.Msg) {
		if err := consumer.handleUserAccountDeleted(ctx, msg); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "user.account_deleted processing failed", slog.String("error", err.Error()))
			_ = msg.Nak()
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "user.account_deleted processed")
		_ = msg.Ack()
	}
	durable := accountDeletedDurableName(instanceID)
	sub, err := js.Subscribe(userAccountDeletedSubject, handler,
		nats.Durable(durable),
		nats.BindStream(userEventsStreamName),
		nats.DeliverAll(),
		nats.ManualAck(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler,
			nats.Bind(userEventsStreamName, durable),
			nats.ManualAck(),
		)
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe user.account_deleted: %w", err)
		}
	}
	return sub, nil
}

func runAccountDeletedConsumer(
	ctx context.Context,
	natsURL, instanceID string,
	profiles accountDeletedProfileLister,
	targets dmPeerDeletionTargetStore,
	publisher dmPeerDeletedPublisher,
	logger *slog.Logger,
) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("user.account_deleted consumer: missing NATS URL")
	}
	for {
		err := runAccountDeletedConsumerOnce(ctx, natsURL, instanceID, profiles, targets, publisher, logger)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if logger != nil {
			logger.Warn("user.account_deleted consumer retrying", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func runAccountDeletedConsumerOnce(
	ctx context.Context,
	natsURL, instanceID string,
	profiles accountDeletedProfileLister,
	targets dmPeerDeletionTargetStore,
	publisher dmPeerDeletedPublisher,
	logger *slog.Logger,
) error {
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-chat-account-deleted"),
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
	sub, err := subscribeAccountDeletedConsumerWithContext(ctx, js, profiles, targets, publisher, instanceID, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("user.account_deleted unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
