package outboxdelivery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

const (
	maxClaimBatch = 100
	chatEvents    = "chat_events"
	spaceUpdated  = "space.updated"
)

var errLeaseTokenMismatch = errors.New("ownership outbox lease token mismatch")

// ClaimedEvent contains immutable outbox data and the current delivery fence.
type ClaimedEvent struct {
	EventID    uuid.UUID
	SpaceID    uuid.UUID
	EventType  string
	CreatedAt  time.Time
	LeaseToken uuid.UUID
}

// DeliveryStore owns claim and acknowledgement persistence. MarkFailed is a
// separate optional capability so narrow contract fakes can still model crash
// boundaries where no known-failure release occurs.
type DeliveryStore interface {
	ClaimReady(context.Context, int) ([]ClaimedEvent, error)
	MarkDelivered(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type failureStore interface {
	MarkFailed(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

// Transport publishes one prepared message and returns the JetStream PubAck.
type Transport interface {
	Publish(context.Context, *nats.Msg) (*nats.PubAck, error)
}

// Coordinator executes one bounded claim/publish/ack delivery pass.
type Coordinator struct {
	store     DeliveryStore
	transport Transport
}

func NewCoordinator(store DeliveryStore, transport Transport) *Coordinator {
	return &Coordinator{store: store, transport: transport}
}

// DispatchOnce publishes every event in one committed claim. Known publish and
// acknowledgement failures release through the store's database retry policy;
// an ambiguous successful publish followed by a mark failure keeps its lease.
func (c *Coordinator) DispatchOnce(ctx context.Context) error {
	if c == nil || c.store == nil || c.transport == nil {
		return errors.New("ownership outbox coordinator not configured")
	}
	if ctx == nil {
		return errors.New("ownership outbox dispatch context is nil")
	}
	claimed, err := c.store.ClaimReady(ctx, maxClaimBatch)
	if err != nil {
		return fmt.Errorf("claim ownership outbox: %w", err)
	}

	var dispatchErrors []error
	for _, event := range claimed {
		if err := ctx.Err(); err != nil {
			dispatchErrors = append(dispatchErrors, err)
			break
		}
		msg, err := messageFor(event)
		if err != nil {
			dispatchErrors = append(dispatchErrors, c.releaseKnownFailure(ctx, event, err))
			continue
		}
		ack, err := c.transport.Publish(ctx, msg)
		if err != nil {
			dispatchErrors = append(dispatchErrors, c.releaseKnownFailure(ctx, event,
				fmt.Errorf("publish ownership outbox event %s: %w", event.EventID, err)))
			continue
		}
		if ack == nil {
			dispatchErrors = append(dispatchErrors, c.releaseKnownFailure(ctx, event,
				fmt.Errorf("publish ownership outbox event %s: missing PubAck", event.EventID)))
			continue
		}
		if ack.Stream != chatEvents || ack.Sequence == 0 {
			dispatchErrors = append(dispatchErrors, c.releaseKnownFailure(ctx, event,
				fmt.Errorf("publish ownership outbox event %s: unexpected PubAck stream %q sequence %d", event.EventID, ack.Stream, ack.Sequence)))
			continue
		}
		marked, err := c.store.MarkDelivered(ctx, event.EventID, event.LeaseToken)
		if err != nil {
			dispatchErrors = append(dispatchErrors,
				fmt.Errorf("mark ownership outbox event %s delivered: %w", event.EventID, err))
			continue
		}
		if !marked {
			dispatchErrors = append(dispatchErrors,
				fmt.Errorf("mark ownership outbox event %s delivered: %w", event.EventID, errLeaseTokenMismatch))
		}
	}
	return errors.Join(dispatchErrors...)
}

func (c *Coordinator) releaseKnownFailure(ctx context.Context, event ClaimedEvent, cause error) error {
	retryStore, ok := c.store.(failureStore)
	if !ok {
		return cause
	}
	released, err := retryStore.MarkFailed(ctx, event.EventID, event.LeaseToken)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("schedule ownership outbox event %s retry: %w", event.EventID, err))
	}
	if !released {
		return errors.Join(cause, fmt.Errorf("schedule ownership outbox event %s retry: %w", event.EventID, errLeaseTokenMismatch))
	}
	return cause
}

func messageFor(event ClaimedEvent) (*nats.Msg, error) {
	if event.EventID == uuid.Nil || event.SpaceID == uuid.Nil || event.LeaseToken == uuid.Nil {
		return nil, errors.New("ownership outbox event has invalid identity or lease")
	}
	if event.EventType != spaceUpdated {
		return nil, fmt.Errorf("unsupported ownership outbox event type %q", event.EventType)
	}
	timestamp := timestamppb.New(event.CreatedAt.UTC())
	if err := timestamp.CheckValid(); err != nil {
		return nil, fmt.Errorf("invalid ownership outbox occurred_at: %w", err)
	}
	envelope := &eventsv1.ChatStreamEvent{
		EventId:    event.EventID.String(),
		OccurredAt: timestamp,
		Payload: &eventsv1.ChatStreamEvent_SpaceUpdated{
			SpaceUpdated: &eventsv1.SpaceUpdated{SpaceId: event.SpaceID.String()},
		},
	}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal ownership ChatStreamEvent: %w", err)
	}
	msg := &nats.Msg{Subject: spaceUpdated, Data: payload, Header: nats.Header{}}
	msg.Header.Set(nats.MsgIdHdr, event.EventID.String())
	return msg, nil
}
