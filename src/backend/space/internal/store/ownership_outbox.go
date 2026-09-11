package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ReadReadyOwnershipOutbox visits immutable ready events in stable delivery order.
// Claiming, acknowledgement, deletion and publishing belong to the dispatcher slice.
func (s *SpaceStore) ReadReadyOwnershipOutbox(ctx context.Context, limit int, visit func(eventID, operationID, spaceID, previousOwnerProfileID, newOwnerProfileID uuid.UUID, eventType string, createdAt time.Time) error) error {
	if limit < 1 || limit > 100 || visit == nil {
		return errors.New("invalid ownership outbox read")
	}
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `SELECT event_id,operation_id,space_id,previous_owner_profile_id,new_owner_profile_id,event_type,created_at
		FROM ownership_outbox WHERE ready ORDER BY created_at,event_id LIMIT $1`, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var eventID, operationID, spaceID, previousOwnerID, newOwnerID uuid.UUID
		var eventType string
		var createdAt time.Time
		if err := rows.Scan(&eventID, &operationID, &spaceID, &previousOwnerID, &newOwnerID, &eventType, &createdAt); err != nil {
			return err
		}
		if err := visit(eventID, operationID, spaceID, previousOwnerID, newOwnerID, eventType, createdAt.UTC()); err != nil {
			return err
		}
	}
	return rows.Err()
}
