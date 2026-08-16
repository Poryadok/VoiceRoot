package subscriptionconsume

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

const streamName = "subscription_events"

// TierCache stores account subscription tiers updated from subscription.events.
type TierCache struct {
	mu    sync.RWMutex
	tiers map[string]string
}

// NewTierCache returns an empty tier cache (unknown accounts resolve to free).
func NewTierCache() *TierCache {
	return &TierCache{tiers: make(map[string]string)}
}

// SetTier records the tier for an account ID.
func (c *TierCache) SetTier(accountID, tier string) {
	if c == nil || strings.TrimSpace(accountID) == "" || strings.TrimSpace(tier) == "" {
		return
	}
	c.mu.Lock()
	c.tiers[strings.ToLower(strings.TrimSpace(accountID))] = strings.ToLower(strings.TrimSpace(tier))
	c.mu.Unlock()
}

// Tier returns the cached tier or "free".
func (c *TierCache) Tier(accountID string) string {
	if c == nil {
		return "free"
	}
	accountID = strings.ToLower(strings.TrimSpace(accountID))
	if accountID == "" {
		return "free"
	}
	c.mu.RLock()
	tier, ok := c.tiers[accountID]
	c.mu.RUnlock()
	if !ok || tier == "" {
		return "free"
	}
	return tier
}

// ApplySubscriptionEvent updates tier cache from a domain event.
func ApplySubscriptionEvent(cache *TierCache, env *eventsv1.SubscriptionStreamEvent) {
	if cache == nil || env == nil {
		return
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.SubscriptionStreamEvent_PlanStarted:
		if started := p.PlanStarted; started != nil {
			cache.SetTier(started.GetAccountId(), tierFromPlan(started.GetPlan()))
		}
	case *eventsv1.SubscriptionStreamEvent_PlanCancelled:
		if cancelled := p.PlanCancelled; cancelled != nil {
			cache.SetTier(cancelled.GetAccountId(), "free")
		}
	case *eventsv1.SubscriptionStreamEvent_PaymentFailed:
			// payment_failed → grace_period: entitlements stay active (docs/features/subscription.md).
			_ = p
		case *eventsv1.SubscriptionStreamEvent_PlanExpired:
		if expired := p.PlanExpired; expired != nil {
			cache.SetTier(expired.GetAccountId(), "free")
		}
	case *eventsv1.SubscriptionStreamEvent_Downgrade:
		if downgrade := p.Downgrade; downgrade != nil {
			cache.SetTier(downgrade.GetAccountId(), "free")
		}
	case *eventsv1.SubscriptionStreamEvent_PaymentSuccess:
		// payment_success alone does not change tier; plan_started is authoritative.
		_ = p
	}
}

func tierFromPlan(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "premium", "space_pro":
		return "premium"
	default:
		return "free"
	}
}

// Run starts a durable JetStream consumer on subscription.> and applies events to cache.
func Run(ctx context.Context, natsURL, durable string, cache *TierCache) error {
	if cache == nil {
		return fmt.Errorf("subscription consumer: missing tier cache")
	}
	url := strings.TrimSpace(natsURL)
	if url == "" {
		return fmt.Errorf("subscription consumer: missing NATS_URL")
	}
	if strings.TrimSpace(durable) == "" {
		durable = "subscription_tier_default"
	}
	nc, err := nats.Connect(url,
		nats.Name("voice-subscription-consumer"),
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
		ApplySubscriptionEvent(cache, &env)
	}

	sub, err := js.Subscribe("subscription.>", handler, nats.Durable(durable), nats.BindStream(streamName), nats.DeliverNew())
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(streamName, durable))
		if err != nil {
			return fmt.Errorf("subscribe subscription.events: %w", err)
		}
	}
	go func() {
		<-ctx.Done()
		_ = sub.Unsubscribe()
	}()
	<-ctx.Done()
	return ctx.Err()
}
