package outboxdelivery

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"voice/backend/space/internal/store"
)

type ownershipOutboxStore interface {
	ClaimReadyOwnershipOutbox(context.Context, int) ([]store.ClaimedOwnershipOutboxEvent, error)
	MarkOwnershipOutboxFailed(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	MarkOwnershipOutboxDelivered(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

// StoreAdapter maps the Space persistence contract to the coordinator's small
// delivery interface.
type StoreAdapter struct {
	store ownershipOutboxStore
}

func NewStoreAdapter(source ownershipOutboxStore) *StoreAdapter {
	return &StoreAdapter{store: source}
}

func (a *StoreAdapter) ClaimReady(ctx context.Context, limit int) ([]ClaimedEvent, error) {
	if a == nil || a.store == nil {
		return nil, errors.New("ownership outbox store adapter not configured")
	}
	rows, err := a.store.ClaimReadyOwnershipOutbox(ctx, limit)
	if err != nil {
		return nil, err
	}
	claimed := make([]ClaimedEvent, len(rows))
	for index, row := range rows {
		claimed[index] = ClaimedEvent{
			EventID:    row.EventID,
			SpaceID:    row.SpaceID,
			EventType:  row.EventType,
			CreatedAt:  row.CreatedAt,
			LeaseToken: row.LeaseToken,
		}
	}
	return claimed, nil
}

func (a *StoreAdapter) MarkFailed(ctx context.Context, eventID, leaseToken uuid.UUID) (bool, error) {
	if a == nil || a.store == nil {
		return false, errors.New("ownership outbox store adapter not configured")
	}
	return a.store.MarkOwnershipOutboxFailed(ctx, eventID, leaseToken)
}

func (a *StoreAdapter) MarkDelivered(ctx context.Context, eventID, leaseToken uuid.UUID) (bool, error) {
	if a == nil || a.store == nil {
		return false, errors.New("ownership outbox store adapter not configured")
	}
	return a.store.MarkOwnershipOutboxDelivered(ctx, eventID, leaseToken)
}
