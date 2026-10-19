package ordinarycall

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Lookup is the ordinary public-read locator. It is not the cross-source A3
// snapshot API and cannot classify global absence or authorize an MM enqueue.
type Lookup interface {
	FindForProfile(context.Context, uuid.UUID) (*Room, error)
	FindActiveGroupForChat(context.Context, uuid.UUID) (*Room, error)
}

func (s *PostgresStore) FindForProfile(ctx context.Context, profile uuid.UUID) (*Room, error) {
	if s == nil || s.pool == nil || profile == uuid.Nil {
		return nil, ErrUnavailable
	}
	rows, err := s.pool.Query(ctx, `SELECT r.room_id,r.document,clock_timestamp(),m.account_id,m.membership_generation,
m.fence_operation_id,m.media_identity,m.state,
EXISTS(SELECT 1 FROM voice_account_voice_fences f WHERE f.account_id=m.account_id AND f.profile_id=m.profile_id
AND f.room_id=m.room_id::text AND f.admission_operation_id=m.fence_operation_id
AND f.admission_generation=m.membership_generation::text AND f.state='active' AND f.reservation_expires_at IS NULL)
FROM voice_ordinary_memberships m JOIN voice_ordinary_rooms r USING(room_id)
WHERE m.profile_id=$1 AND m.state IN ('PENDING','RESERVED','JOINED','LEAVING') AND r.state<>'CLOSED'`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result *Room
	for rows.Next() {
		if result != nil {
			return nil, ErrConflict
		}
		var roomID, account, generation, operation uuid.UUID
		var body []byte
		var observedAt time.Time
		var identity, state string
		var ownsFence bool
		if err := rows.Scan(&roomID, &body, &observedAt, &account, &generation, &operation, &identity, &state, &ownsFence); err != nil {
			return nil, err
		}
		r, err := decodeRoom(body, roomID)
		if err != nil {
			return nil, err
		}
		member, exists := r.Members[profile.String()]
		if !exists || !ownsFence || member.AccountID != account || member.Generation != generation ||
			member.FenceOperationID != operation || member.MediaIdentity != identity || string(member.State) != state {
			return nil, ErrUnavailable
		}
		r.TransactionTime = observedAt
		result = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrNotFound
	}
	return result, nil
}

func (s *PostgresStore) FindActiveGroupForChat(ctx context.Context, chat uuid.UUID) (*Room, error) {
	if s == nil || s.pool == nil || chat == uuid.Nil {
		return nil, ErrUnavailable
	}
	var roomID uuid.UUID
	var body []byte
	var observedAt time.Time
	if err := s.pool.QueryRow(ctx, `SELECT room_id,document,clock_timestamp() FROM voice_ordinary_rooms
WHERE is_group AND chat_id=$1 AND state<>'CLOSED'`, chat).Scan(&roomID, &body, &observedAt); err != nil {
		return nil, mapStoreError(err)
	}
	r, err := decodeRoom(body, roomID)
	if err != nil {
		return nil, err
	}
	if !r.Intent.Group || r.Intent.ChatID != chat {
		return nil, ErrUnavailable
	}
	r.TransactionTime = observedAt
	return r, nil
}
