package subscriptionconsume

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/space/internal/store"
)

const streamName = "subscription_events"
const defaultDurable = "space_subscription_entitlement"
const subjectSpaceProStarted = "subscription.space_pro_started"
const subjectSpaceProExpired = "subscription.space_pro_expired"
const spaceDeliverySubject = "_INBOX.voice.space.space_subscription_entitlement"

var errInvalidSubscriptionEvent = errors.New("invalid space entitlement event")

type Consumer struct {
	nc  *nats.Conn
	sub *nats.Subscription
}

func validateSpaceDurable(info *nats.ConsumerInfo) error {
	if info == nil || info.Stream != streamName || info.Name != defaultDurable ||
		info.Config.Durable != defaultDurable || info.Config.FilterSubject != "" ||
		len(info.Config.FilterSubjects) != 2 || info.Config.DeliverSubject != spaceDeliverySubject ||
		info.Config.DeliverGroup != "" || info.Config.DeliverPolicy != nats.DeliverNewPolicy ||
		info.Config.AckPolicy != nats.AckExplicitPolicy {
		return fmt.Errorf("space entitlement durable %q has incompatible configuration", defaultDurable)
	}
	seen := map[string]bool{}
	for _, subject := range info.Config.FilterSubjects {
		seen[subject] = true
	}
	if !seen[subjectSpaceProStarted] || !seen[subjectSpaceProExpired] || len(seen) != 2 {
		return fmt.Errorf("space entitlement durable %q has incompatible subjects", defaultDurable)
	}
	return nil
}

// SpaceEntitlementStore syncs Space Pro cache rows in space_db.
type SpaceEntitlementStore interface {
	UpsertSpaceSubscription(ctx context.Context, spaceID, purchaserAccountID uuid.UUID, status string) error
	FinalizeSpacePro(ctx context.Context, spaceID uuid.UUID) error
}

// SpaceStoreEntitlement adapts store.SpaceStore to SpaceEntitlementStore.
type SpaceStoreEntitlement struct {
	Store *store.SpaceStore
}

func (a *SpaceStoreEntitlement) UpsertSpaceSubscription(ctx context.Context, spaceID, purchaserAccountID uuid.UUID, status string) error {
	if a == nil || a.Store == nil {
		return fmt.Errorf("space store not configured")
	}
	return a.Store.UpsertSpaceSubscription(ctx, spaceID, purchaserAccountID, status)
}

func (a *SpaceStoreEntitlement) FinalizeSpacePro(ctx context.Context, spaceID uuid.UUID) error {
	if a == nil || a.Store == nil {
		return fmt.Errorf("space store not configured")
	}
	return a.Store.FinalizeSpacePro(ctx, spaceID)
}

// ApplySubscriptionEvent mirrors Space Pro entitlement events into space_db.
func ApplySubscriptionEvent(entitlements SpaceEntitlementStore, env *eventsv1.SubscriptionStreamEvent) error {
	if entitlements == nil || env == nil {
		return errInvalidSubscriptionEvent
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.SubscriptionStreamEvent_SpaceProStarted:
		started := p.SpaceProStarted
		if started == nil {
			return errInvalidSubscriptionEvent
		}
		spaceID, err := uuid.Parse(strings.TrimSpace(started.GetSpaceId()))
		if err != nil {
			return fmt.Errorf("%w: invalid space ID: %v", errInvalidSubscriptionEvent, err)
		}
		purchaserID, err := uuid.Parse(strings.TrimSpace(started.GetPurchaserAccountId()))
		if err != nil {
			return fmt.Errorf("%w: invalid purchaser account ID: %v", errInvalidSubscriptionEvent, err)
		}
		return entitlements.UpsertSpaceSubscription(context.Background(), spaceID, purchaserID, "active")
	case *eventsv1.SubscriptionStreamEvent_SpaceProExpired:
		expired := p.SpaceProExpired
		if expired == nil {
			return errInvalidSubscriptionEvent
		}
		spaceID, err := uuid.Parse(strings.TrimSpace(expired.GetSpaceId()))
		if err != nil {
			return fmt.Errorf("%w: invalid space ID: %v", errInvalidSubscriptionEvent, err)
		}
		return entitlements.FinalizeSpacePro(context.Background(), spaceID)
	}
	return errInvalidSubscriptionEvent
}

// Start validates and binds the preprovisioned Space Pro durable.
func Start(ctx context.Context, natsURL, durable string, entitlements SpaceEntitlementStore) (*Consumer, error) {
	if entitlements == nil {
		return nil, fmt.Errorf("space entitlement store required")
	}
	url := strings.TrimSpace(natsURL)
	if url == "" {
		return nil, fmt.Errorf("missing NATS_URL")
	}
	if strings.TrimSpace(durable) == "" {
		durable = defaultDurable
	}
	if durable != defaultDurable {
		return nil, fmt.Errorf("unsupported space entitlement durable %q", durable)
	}
	nc, err := nats.Connect(url,
		nats.Name("voice-space-subscription-consumer"),
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
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	info, err := js.ConsumerInfo(streamName, durable)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("inspect space entitlement durable: %w", err)
	}
	if err := validateSpaceDurable(info); err != nil {
		nc.Close()
		return nil, err
	}

	handler := func(msg *nats.Msg) {
		var env eventsv1.SubscriptionStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			_ = msg.Term()
			return
		}
		if err := ApplySubscriptionEvent(entitlements, &env); err != nil {
			if errors.Is(err, errInvalidSubscriptionEvent) {
				_ = msg.Term()
				return
			}
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	}

	sub, err := js.Subscribe("", handler, nats.Bind(streamName, durable), nats.ManualAck())
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("bind space subscription events: %w", err)
	}
	return &Consumer{nc: nc, sub: sub}, nil
}

func (c *Consumer) Close() {
	if c != nil && c.nc != nil {
		_ = c.nc.Drain()
	}
}
func (c *Consumer) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// Run binds and processes Space Pro entitlements until cancellation.
func Run(ctx context.Context, natsURL, durable string, entitlements SpaceEntitlementStore) error {
	c, err := Start(ctx, natsURL, durable, entitlements)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Run(ctx)
}
