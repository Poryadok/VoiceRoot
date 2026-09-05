package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/natslog"
	"voice/backend/pkg/runtimeconfig"
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

func accountDeletedDeliverySubject(durable string) string {
	return "_INBOX.voice.chat." + durable
}

func accountDeletedConsumerConfig(durable string) *nats.ConsumerConfig {
	return &nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: accountDeletedDeliverySubject(durable),
		DeliverPolicy:  nats.DeliverAllPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
		FilterSubject:  userAccountDeletedSubject,
	}
}

// ensureAccountDeletedDurable creates the durable outside of Subscribe so
// nats.go does not own its lifecycle. Existing legacy durables retain their
// delivery subject and acknowledgement cursor; rebinding them is safe once
// their contract is validated.
func ensureAccountDeletedDurable(js nats.JetStreamContext, durable string) error {
	info, err := js.ConsumerInfo(userEventsStreamName, durable)
	if errors.Is(err, nats.ErrConsumerNotFound) {
		if _, err := js.AddConsumer(userEventsStreamName, accountDeletedConsumerConfig(durable)); err != nil {
			return fmt.Errorf("create user.account_deleted durable: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect user.account_deleted durable: %w", err)
	}
	if info.Config.Durable != durable ||
		info.Config.DeliverPolicy != nats.DeliverAllPolicy ||
		info.Config.AckPolicy != nats.AckExplicitPolicy ||
		info.Config.FilterSubject != userAccountDeletedSubject ||
		info.Config.DeliverSubject == "" {
		return fmt.Errorf("user.account_deleted durable %q has incompatible configuration", durable)
	}
	if _, err := js.UpdateConsumer(userEventsStreamName, &info.Config); err != nil {
		return fmt.Errorf("update user.account_deleted durable: %w", err)
	}
	return nil
}

type accountDeletedInFlight struct {
	mu       sync.Mutex
	stopping bool
	wg       sync.WaitGroup
}

func (f *accountDeletedInFlight) begin() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopping {
		return false
	}
	f.wg.Add(1)
	return true
}

func (f *accountDeletedInFlight) done() {
	f.wg.Done()
}

func (f *accountDeletedInFlight) stop() {
	f.mu.Lock()
	f.stopping = true
	f.mu.Unlock()
}

func (f *accountDeletedInFlight) wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		f.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for in-flight user.account_deleted handlers: %w", ctx.Err())
	}
}

type accountDeletedSubscription struct {
	sub      *nats.Subscription
	inFlight *accountDeletedInFlight
}

func (s *accountDeletedSubscription) stopIntakeAndWait(ctx context.Context) error {
	if s == nil || s.sub == nil || s.inFlight == nil {
		return fmt.Errorf("user.account_deleted subscription not configured")
	}
	s.inFlight.stop()
	var shutdownErrors []error
	if err := s.sub.Unsubscribe(); err != nil {
		shutdownErrors = append(shutdownErrors, fmt.Errorf("unsubscribe user.account_deleted: %w", err))
	}
	if err := s.inFlight.wait(ctx); err != nil {
		shutdownErrors = append(shutdownErrors, err)
	}
	return errors.Join(shutdownErrors...)
}

func subscribeAccountDeletedConsumer(
	js nats.JetStreamContext,
	profiles accountDeletedProfileLister,
	targets dmPeerDeletionTargetStore,
	publisher dmPeerDeletedPublisher,
	instanceID string,
	logger *slog.Logger,
) (*nats.Subscription, error) {
	subscription, err := subscribeAccountDeletedConsumerWithContext(context.Background(), js, profiles, targets, publisher, instanceID, logger)
	if err != nil {
		return nil, err
	}
	return subscription.sub, nil
}

func subscribeAccountDeletedConsumerWithContext(
	ctx context.Context,
	js nats.JetStreamContext,
	profiles accountDeletedProfileLister,
	targets dmPeerDeletionTargetStore,
	publisher dmPeerDeletedPublisher,
	instanceID string,
	logger *slog.Logger,
) (*accountDeletedSubscription, error) {
	if js == nil {
		return nil, fmt.Errorf("user.account_deleted consumer JetStream not configured")
	}
	durable := accountDeletedDurableName(instanceID)
	if err := ensureAccountDeletedDurable(js, durable); err != nil {
		return nil, err
	}
	consumer := newAccountDeletedDMConsumer(profiles, targets, publisher, logger)
	inFlight := &accountDeletedInFlight{}
	handler := func(msg *nats.Msg) {
		if !inFlight.begin() {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "user.account_deleted intake stopped")
			_ = msg.Nak()
			return
		}
		defer inFlight.done()
		if err := consumer.handleUserAccountDeleted(ctx, msg); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "user.account_deleted processing failed", slog.String("error", err.Error()))
			_ = msg.Nak()
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "user.account_deleted processed")
		_ = msg.Ack()
	}
	sub, err := js.Subscribe(userAccountDeletedSubject, handler,
		nats.Bind(userEventsStreamName, durable),
		nats.ManualAck(),
	)
	if err != nil {
		return nil, fmt.Errorf("bind user.account_deleted durable: %w", err)
	}
	return &accountDeletedSubscription{sub: sub, inFlight: inFlight}, nil
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

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return fmt.Errorf("jetstream: %w", err)
	}
	handlerCtx, cancelHandler := context.WithCancel(context.Background())
	defer cancelHandler()
	sub, err := subscribeAccountDeletedConsumerWithContext(handlerCtx, js, profiles, targets, publisher, instanceID, logger)
	if err != nil {
		nc.Close()
		return err
	}

	<-ctx.Done()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
	defer cancelShutdown()
	if err := sub.stopIntakeAndWait(shutdownCtx); err != nil {
		if logger != nil {
			logger.Warn("user.account_deleted graceful shutdown failed", slog.String("error", err.Error()))
		}
		nc.Close()
		return ctx.Err()
	}
	if err := nc.Drain(); err != nil {
		if logger != nil {
			logger.Warn("user.account_deleted NATS drain failed", slog.String("error", err.Error()))
		}
		nc.Close()
	}
	return ctx.Err()
}
