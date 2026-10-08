package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const voiceInvalidationOutboxPageSize = 250

type VoiceAccessInvalidation struct {
	EventID   uuid.UUID
	SpaceID   uuid.UUID
	Epoch     uint64
	RoomID    *uuid.UUID
	ProfileID *uuid.UUID
	CreatedAt time.Time
}

// ReadLatestVoiceAccessInvalidations returns the newest immutable snapshot for
// each Space after the cursor. Replaying after process restart is safe because
// event IDs remain stable and Voice reconciles against current authority.
func (s *SpaceStore) ReadLatestVoiceAccessInvalidations(ctx context.Context, afterSpaceID uuid.UUID, limit int) ([]VoiceAccessInvalidation, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if limit <= 0 || limit > voiceInvalidationOutboxPageSize {
		limit = voiceInvalidationOutboxPageSize
	}
	rows, err := s.Pool.Query(ctx, `
SELECT event_id,space_id,access_epoch,voice_room_id,profile_id,created_at
FROM (
    SELECT DISTINCT ON (space_id) event_id,space_id,access_epoch,voice_room_id,profile_id,created_at
    FROM space_voice_access_outbox
    WHERE space_id>$1
    ORDER BY space_id,access_epoch DESC
) latest
ORDER BY space_id
LIMIT $2
`, afterSpaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]VoiceAccessInvalidation, 0, limit)
	for rows.Next() {
		var event VoiceAccessInvalidation
		var roomID, profileID pgtype.UUID
		var epoch int64
		if err := rows.Scan(&event.EventID, &event.SpaceID, &epoch, &roomID, &profileID, &event.CreatedAt); err != nil {
			return nil, err
		}
		if event.EventID == uuid.Nil || event.SpaceID == uuid.Nil || epoch <= 0 {
			return nil, errors.New("space store: invalid Voice access outbox row")
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
