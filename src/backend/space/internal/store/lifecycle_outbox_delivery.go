package store

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"time"
	"voice/backend/space/internal/spacecore"
)

func (s *SpaceStore) LifecycleEventPurgeDecision(ctx context.Context, record spacecore.LifecycleOutboxRecord) (time.Time, error) {
	if s == nil || s.Pool == nil || record.EventType != "space.deleted" {
		return time.Time{}, ErrLifecycleEvidenceInvalid
	}
	var decision time.Time
	err := s.Pool.QueryRow(ctx, `SELECT purge_decided_at FROM space_lifecycle_outbox WHERE event_id=$1 AND space_id=$2 AND deletion_operation_id=$3 AND generation=$4 AND event_type='space.deleted' AND state='READY'`, record.EventID, record.SpaceID, record.DeletionOperationID, record.Generation).Scan(&decision)
	if err != nil {
		return time.Time{}, err
	}
	return decision.UTC(), nil
}

func (s *SpaceStore) MarkLifecycleEventDelivered(ctx context.Context, record spacecore.LifecycleOutboxRecord) error {
	if s == nil || s.Pool == nil {
		return ErrLifecycleEvidenceInvalid
	}
	raw := marshalLifecycleOutbox(record)
	hash := lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", raw)
	var saved []byte
	err := s.Pool.QueryRow(ctx, `UPDATE space_lifecycle_outbox SET state='DELIVERED',delivered_at=COALESCE(delivered_at,clock_timestamp()) WHERE event_id=$1 AND state IN ('READY','DELIVERED') AND event_bytes=$2 AND event_sha256=$3 RETURNING event_bytes`, uuid.MustParse(record.EventID), raw, hash[:]).Scan(&saved)
	if err != nil {
		return err
	}
	if !bytes.Equal(saved, raw) {
		return ErrLifecycleConflict
	}
	return nil
}
