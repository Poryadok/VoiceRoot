package subscriptionevents

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
	streamName                 = "subscription_events"
	subjectPlanStarted         = "subscription.plan_started"
	subjectPlanCancelled       = "subscription.plan_cancelled"
	subjectPlanExpired         = "subscription.plan_expired"
	subjectDowngrade           = "subscription.downgrade"
	subjectPaymentSuccess      = "subscription.payment_success"
	subjectPaymentFailed       = "subscription.payment_failed"
	subjectSpaceProStarted     = "subscription.space_pro_started"
	subjectSpaceProExpired     = "subscription.space_pro_expired"
	subjectGraceReminder       = "subscription.grace_reminder"
)

// Publisher publishes subscription.events domain payloads.
type Publisher interface {
	PublishPlanStarted(ctx context.Context, accountID, plan string) error
	PublishPlanCancelled(ctx context.Context, accountID, plan string) error
	PublishPlanExpired(ctx context.Context, accountID, plan string) error
	PublishDowngrade(ctx context.Context, accountID, plan string) error
	PublishPaymentSuccess(ctx context.Context, accountID, provider string) error
	PublishPaymentFailed(ctx context.Context, accountID, provider string) error
	PublishSpaceProStarted(ctx context.Context, spaceID, purchaserAccountID string) error
	PublishSpaceProExpired(ctx context.Context, spaceID string) error
	PublishGraceReminder(ctx context.Context, accountID, plan string, day int32) error
	Close() error
}

// JetStreamPublisher publishes SubscriptionStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS and prepares JetStream for subscription.events.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-subscription-events"),
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

func subscriptionStreamSubjects() []string {
	return []string{
		subjectPlanStarted,
		subjectPlanCancelled,
		subjectPlanExpired,
		subjectDowngrade,
		subjectPaymentSuccess,
		subjectPaymentFailed,
		subjectSpaceProStarted,
		subjectSpaceProExpired,
		subjectGraceReminder,
	}
}

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		desired := subscriptionStreamSubjects()
		info, err := p.js.StreamInfo(streamName)
		if err != nil {
			_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
				Name:      streamName,
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

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.SubscriptionStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal SubscriptionStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "subscription event published",
		slog.String("event_id", env.GetEventId()),
	)
	return nil
}

func newSubscriptionEvent() *eventsv1.SubscriptionStreamEvent {
	return &eventsv1.SubscriptionStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
	}
}

func (p *JetStreamPublisher) PublishPlanStarted(ctx context.Context, accountID, plan string) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_PlanStarted{
		PlanStarted: &eventsv1.PlanStarted{AccountId: accountID, Plan: plan},
	}
	return p.publishProto(ctx, subjectPlanStarted, env)
}

func (p *JetStreamPublisher) PublishPlanCancelled(ctx context.Context, accountID, plan string) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_PlanCancelled{
		PlanCancelled: &eventsv1.PlanCancelled{AccountId: accountID, Plan: plan},
	}
	return p.publishProto(ctx, subjectPlanCancelled, env)
}

func (p *JetStreamPublisher) PublishPaymentSuccess(ctx context.Context, accountID, provider string) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_PaymentSuccess{
		PaymentSuccess: &eventsv1.PaymentSuccess{AccountId: accountID, Provider: provider},
	}
	return p.publishProto(ctx, subjectPaymentSuccess, env)
}

func (p *JetStreamPublisher) PublishPaymentFailed(ctx context.Context, accountID, provider string) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_PaymentFailed{
		PaymentFailed: &eventsv1.PaymentFailed{AccountId: accountID, Provider: provider},
	}
	return p.publishProto(ctx, subjectPaymentFailed, env)
}

func (p *JetStreamPublisher) PublishPlanExpired(ctx context.Context, accountID, plan string) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_PlanExpired{
		PlanExpired: &eventsv1.PlanExpired{AccountId: accountID, Plan: plan},
	}
	return p.publishProto(ctx, subjectPlanExpired, env)
}

func (p *JetStreamPublisher) PublishDowngrade(ctx context.Context, accountID, plan string) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_Downgrade{
		Downgrade: &eventsv1.Downgrade{AccountId: accountID, Plan: plan},
	}
	return p.publishProto(ctx, subjectDowngrade, env)
}

func (p *JetStreamPublisher) PublishSpaceProStarted(ctx context.Context, spaceID, purchaserAccountID string) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_SpaceProStarted{
		SpaceProStarted: &eventsv1.SpaceProStarted{SpaceId: spaceID, PurchaserAccountId: purchaserAccountID},
	}
	return p.publishProto(ctx, subjectSpaceProStarted, env)
}

func (p *JetStreamPublisher) PublishSpaceProExpired(ctx context.Context, spaceID string) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_SpaceProExpired{
		SpaceProExpired: &eventsv1.SpaceProExpired{SpaceId: spaceID},
	}
	return p.publishProto(ctx, subjectSpaceProExpired, env)
}

func (p *JetStreamPublisher) PublishGraceReminder(ctx context.Context, accountID, plan string, day int32) error {
	env := newSubscriptionEvent()
	env.Payload = &eventsv1.SubscriptionStreamEvent_GraceReminder{
		GraceReminder: &eventsv1.GraceReminder{
			AccountId: accountID,
			Plan:      plan,
			Day:       day,
		},
	}
	return p.publishProto(ctx, subjectGraceReminder, env)
}

// Close drains the NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
