package ordinarycall

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MediaBinding is private native evidence, never a client-supplied locator.
// Every field binds authorization to one immutable ordinary membership.
type MediaBinding struct {
	RoomID, RoomOperationID, AccountID, ProfileID, Generation, FenceOperationID uuid.UUID
	SessionEpoch                                                                int64
	MediaIdentity                                                               string
}

func (r *Room) MediaBinding(profile uuid.UUID) (MediaBinding, error) {
	if r == nil || !r.CanIssueToken(profile) {
		return MediaBinding{}, ErrNotMember
	}
	m := r.Members[profile.String()]
	return MediaBinding{RoomID: r.Intent.RoomID, RoomOperationID: r.Intent.OperationID,
		AccountID: m.AccountID, ProfileID: m.ProfileID, Generation: m.Generation, FenceOperationID: m.FenceOperationID,
		SessionEpoch: m.SessionEpoch, MediaIdentity: m.MediaIdentity}, nil
}

type MediaAuthorizer interface {
	AuthorizeMedia(context.Context, MediaBinding) (string, error)
}

// AuthorizeMedia checks the final native gate under the same identity barrier as
// conversion. Auth/current-epoch IO happens before this transaction; no external
// service or SFU call is made while holding its room or identity locks.
func (s *PostgresStore) AuthorizeMedia(ctx context.Context, expected MediaBinding) (string, error) {
	if s == nil || s.pool == nil || expected.RoomID == uuid.Nil || expected.RoomOperationID == uuid.Nil || expected.ProfileID == uuid.Nil ||
		expected.AccountID == uuid.Nil || expected.Generation == uuid.Nil || expected.FenceOperationID == uuid.Nil || expected.MediaIdentity == "" || expected.SessionEpoch < 0 {
		return "", ErrInvalidIntent
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", err
	}
	defer rollback(tx)
	var body []byte
	if err := tx.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1 FOR UPDATE`, expected.RoomID).Scan(&body); err != nil {
		return "", mapStoreError(err)
	}
	r, err := decodeRoom(body, expected.RoomID)
	if err != nil {
		return "", err
	}
	if err := lockAdmissions(ctx, tx, r, r); err != nil {
		return "", err
	}
	current, err := r.MediaBinding(expected.ProfileID)
	if err != nil {
		return "", err
	}
	if current != expected {
		return "", ErrStaleGeneration
	}
	var fenced bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_sdk_conversion_fence_receipts
WHERE (state IN ('pending','fenced') AND (source_profile_id=$1 OR target_profile_id=$1 OR target_account_id=$2))
OR (state='activated' AND source_profile_id=$1))`, expected.ProfileID, expected.AccountID).Scan(&fenced); err != nil {
		return "", err
	}
	if fenced {
		return "", ErrConflict
	}
	m := r.Members[expected.ProfileID.String()]
	var owns bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_account_voice_fences f
JOIN voice_profile_account_mappings p ON p.profile_id=f.profile_id AND p.account_id=f.account_id
WHERE f.account_id=$1 AND f.profile_id=$2 AND f.room_id=$3 AND f.admission_operation_id=$4 AND f.admission_generation=$5
AND f.state='active' AND f.reservation_expires_at IS NULL)`,
		m.AccountID, m.ProfileID, expected.RoomID.String(), m.FenceOperationID, m.Generation.String()).Scan(&owns); err != nil {
		return "", err
	}
	if !owns {
		return "", ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return r.LiveKitRoomName, nil
}
