package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const voicePolicyInvalidationOutboxPageSize = 250

type VoicePolicyInvalidation struct {
	EventID   uuid.UUID
	SpaceID   uuid.UUID
	Epoch     uint64
	RoomID    *uuid.UUID
	ProfileID *uuid.UUID
	CreatedAt time.Time
}

// ReadLatestVoicePolicyInvalidations returns the newest immutable policy
// snapshot for each Space. Voice's independent reconciled watermark makes
// replay and epoch gaps safe.
func (s *RoleStore) ReadLatestVoicePolicyInvalidations(ctx context.Context, afterSpaceID uuid.UUID, limit int) ([]VoicePolicyInvalidation, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	if limit <= 0 || limit > voicePolicyInvalidationOutboxPageSize {
		limit = voicePolicyInvalidationOutboxPageSize
	}
	rows, err := s.Pool.Query(ctx, `
SELECT event_id,space_id,policy_epoch,voice_room_id,profile_id,created_at
FROM (
    SELECT DISTINCT ON (space_id) event_id,space_id,policy_epoch,voice_room_id,profile_id,created_at
    FROM role_voice_policy_outbox
    WHERE space_id>$1
    ORDER BY space_id,policy_epoch DESC
) latest
ORDER BY space_id
LIMIT $2
`, afterSpaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]VoicePolicyInvalidation, 0, limit)
	for rows.Next() {
		var event VoicePolicyInvalidation
		var roomID, profileID pgtype.UUID
		var epoch int64
		if err := rows.Scan(&event.EventID, &event.SpaceID, &epoch, &roomID, &profileID, &event.CreatedAt); err != nil {
			return nil, err
		}
		if event.EventID == uuid.Nil || event.SpaceID == uuid.Nil || epoch <= 0 {
			return nil, errors.New("role store: invalid Voice policy outbox row")
		}
		event.Epoch = uint64(epoch)
		if roomID.Valid {
			value := uuid.UUID(roomID.Bytes)
			event.RoomID = &value
		}
		if profileID.Valid {
			value := uuid.UUID(profileID.Bytes)
			event.ProfileID = &value
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
