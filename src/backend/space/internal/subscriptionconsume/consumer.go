package subscriptionconsume

import (
	"context"
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

// Run starts a durable JetStream consumer that mirrors Space Pro entitlements into space_db.
func Run(ctx context.Context, natsURL, durable string, entitlements SpaceEntitlementStore) error {
	if entitlements == nil {
		return fmt.Errorf("space entitlement store required")
	}
	url := strings.TrimSpace(natsURL)
	if url == "" {
		return fmt.Errorf("missing NATS_URL")
	}
	if strings.TrimSpace(durable) == "" {
		durable = "space_subscription_entitlement"
	}
	nc, err := nats.Connect(url,
		nats.Name("voice-space-subscription-consumer"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	go func() {
		<-ctx.Done()
		_ = nc.Drain()
	}()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	handler := func(msg *nats.Msg) {
		var env eventsv1.SubscriptionStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			return
		}
		switch p := env.GetPayload().(type) {
		case *eventsv1.SubscriptionStreamEvent_SpaceProStarted:
			started := p.SpaceProStarted
			if started == nil {
				return
			}
			spaceID, err := uuid.Parse(strings.TrimSpace(started.GetSpaceId()))
			if err != nil {
				return
			}
			purchaserID, err := uuid.Parse(strings.TrimSpace(started.GetPurchaserAccountId()))
			if err != nil {
				return
			}
			_ = entitlements.UpsertSpaceSubscription(context.Background(), spaceID, purchaserID, "active")
		case *eventsv1.SubscriptionStreamEvent_SpaceProExpired:
			expired := p.SpaceProExpired
			if expired == nil {
				return
			}
			spaceID, err := uuid.Parse(strings.TrimSpace(expired.GetSpaceId()))
			if err != nil {
				return
			}
			_ = entitlements.FinalizeSpacePro(context.Background(), spaceID)
		}
	}

	sub, err := js.Subscribe("subscription.space_pro_*", handler, nats.Durable(durable), nats.BindStream(streamName), nats.DeliverNew())
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(streamName, durable))
		if err != nil {
			return fmt.Errorf("subscribe space subscription events: %w", err)
		}
	}
	go func() {
		<-ctx.Done()
		_ = sub.Unsubscribe()
	}()
	<-ctx.Done()
	return ctx.Err()
}
