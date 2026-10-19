package store

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
)

type MemberAdmissionEvent struct {
	EventID, OperationID, SpaceID, ProfileID uuid.UUID
	Generation                               uint64
	OccurredAt                               time.Time
}

func (s *SpaceStore) ListPendingMemberAdmissionEvents(ctx context.Context, limit int) ([]MemberAdmissionEvent, error) {
	if s == nil || s.Pool == nil || s.tx != nil || limit < 1 || limit > 100 {
		return nil, ErrMemberAdmissionConflict
	}
	rows, err := s.Pool.Query(ctx, `SELECT event_id,operation_id,space_id,profile_id,generation,occurred_at
	 FROM space_member_admission_outbox WHERE delivered_at IS NULL ORDER BY occurred_at,event_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []MemberAdmissionEvent
	for rows.Next() {
		var event MemberAdmissionEvent
		if err = rows.Scan(&event.EventID, &event.OperationID, &event.SpaceID, &event.ProfileID, &event.Generation, &event.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// MarkMemberAdmissionEventDelivered follows a validated JetStream PubAck outside
// DB locks. The exact immutable tuple and first ack are checked under the Space
// fence, so purge cannot race a successful durable acknowledgement.
func (s *SpaceStore) MarkMemberAdmissionEventDelivered(ctx context.Context, event MemberAdmissionEvent, stream string, sequence uint64) error {
	if s == nil || s.Pool == nil || s.tx != nil || stream != "chat_events" || sequence == 0 || sequence > math.MaxInt64 || event.EventID == uuid.Nil || event.OperationID == uuid.Nil || event.SpaceID == uuid.Nil || event.ProfileID == uuid.Nil || event.Generation == 0 || event.Generation > math.MaxInt64 || event.OccurredAt.IsZero() {
		return ErrMemberAdmissionConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockLifecycleSpace(ctx, tx, event.SpaceID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE space_member_admission_outbox SET delivered_at=COALESCE(delivered_at,clock_timestamp()),delivery_stream=$7,delivery_sequence=$8
	 WHERE event_id=$1 AND operation_id=$2 AND space_id=$3 AND profile_id=$4 AND generation=$5 AND occurred_at=$6
	 AND (delivered_at IS NULL OR (delivery_stream=$7 AND delivery_sequence=$8))`, event.EventID, event.OperationID, event.SpaceID, event.ProfileID, event.Generation, event.OccurredAt, stream, sequence)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMemberAdmissionConflict
	}
	return tx.Commit(ctx)
}
