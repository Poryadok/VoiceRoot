package gameprovision

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Version records positive observation of the native seam, including absence.
// A legacy pending receipt with NULL is unknown, never a new empty snapshot.
type nativeConversionSnapshot struct {
	Version        int
	Source         *NativeOrdinaryConversionSource
	SourceRoom     string
	TargetConflict bool
}

func snapshotNativeConversion(ctx context.Context, tx pgx.Tx, ids conversionFenceIDSet, legacyRoom string) (nativeConversionSnapshot, error) {
	snap := nativeConversionSnapshot{Version: 1}
	var mismatched bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_profile_account_mappings
WHERE (profile_id=$1 AND account_id<>$2) OR (profile_id=$3 AND account_id<>$4))`,
		ids.sourceProfile, ids.sourceAccount, ids.targetProfile, ids.targetAccount).Scan(&mismatched); err != nil {
		return snap, err
	}
	if mismatched {
		return snap, ErrConflict
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_account_voice_fences
WHERE account_id=$1 AND (state='active' OR reservation_expires_at>clock_timestamp()))`, ids.targetAccount).Scan(&snap.TargetConflict); err != nil {
		return snap, err
	}
	var targetNative bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_ordinary_memberships m JOIN voice_ordinary_rooms r USING(room_id)
WHERE m.account_id=$1 AND m.state IN ('PENDING','RESERVED','JOINED','LEAVING') AND r.state<>'CLOSED')`, ids.targetAccount).Scan(&targetNative); err != nil {
		return snap, err
	}
	snap.TargetConflict = snap.TargetConflict || targetNative
	// A native membership without its exact shared fence is corruption/unknown
	// cleanup, not evidence of absence. All these reads are nonlocking MVCC reads.
	var orphan bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_ordinary_memberships m
JOIN voice_ordinary_rooms r USING(room_id) WHERE m.account_id=$1 AND m.profile_id=$2
AND m.state IN ('PENDING','RESERVED','JOINED','LEAVING') AND r.state<>'CLOSED'
AND NOT EXISTS(SELECT 1 FROM voice_account_voice_fences f WHERE f.account_id=m.account_id
AND f.profile_id=m.profile_id AND f.room_id=m.room_id::text AND f.admission_operation_id=m.fence_operation_id
AND f.admission_generation=m.membership_generation::text AND f.state='active' AND f.reservation_expires_at IS NULL))`, ids.sourceAccount, ids.sourceProfile).Scan(&orphan); err != nil {
		return snap, err
	}
	if orphan {
		return snap, ErrConflict
	}
	var profile uuid.UUID
	var room string
	err := tx.QueryRow(ctx, `SELECT profile_id,room_id FROM voice_account_voice_fences
WHERE account_id=$1 AND (state='active' OR reservation_expires_at>clock_timestamp())`, ids.sourceAccount).Scan(&profile, &room)
	if errors.Is(err, pgx.ErrNoRows) {
		// Stale Redis ordinary records cannot manufacture a new cleanup binding.
		if legacyRoom != "" {
			var native bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_ordinary_rooms WHERE room_id::text=$1 AND state='CLOSED')`, legacyRoom).Scan(&native); err != nil {
				return snap, err
			}
			if !native {
				return snap, ErrConflict
			}
		}
		return snap, nil
	}
	if err != nil {
		return snap, err
	}
	if profile != ids.sourceProfile {
		return snap, ErrConflict
	}
	snap.SourceRoom = room
	var native bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_ordinary_rooms WHERE room_id::text=$1)`, room).Scan(&native); err != nil {
		return snap, err
	}
	if !native {
		if legacyRoom != room {
			return snap, ErrConflict
		}
		return snap, nil
	}
	b := &NativeOrdinaryConversionSource{}
	err = tx.QueryRow(ctx, `SELECT r.room_id,r.operation_id,m.account_id,m.profile_id,m.membership_generation,m.fence_operation_id,m.media_identity
FROM voice_account_voice_fences f JOIN voice_ordinary_rooms r ON r.room_id::text=f.room_id
JOIN voice_ordinary_memberships m ON m.room_id=r.room_id AND m.profile_id=f.profile_id
WHERE f.account_id=$1 AND f.profile_id=$2 AND f.admission_operation_id=m.fence_operation_id
AND f.admission_generation=m.membership_generation::text AND f.account_id=m.account_id
AND f.state='active' AND f.reservation_expires_at IS NULL AND r.state<>'CLOSED'
AND m.state IN ('PENDING','RESERVED','JOINED','LEAVING')
AND r.document->'intent'->>'operation_id'=r.operation_id::text
AND r.document->'members'->($2::uuid::text)->>'generation'=m.membership_generation::text
AND r.document->'members'->($2::uuid::text)->>'fence_operation_id'=m.fence_operation_id::text
AND r.document->'members'->($2::uuid::text)->>'account_id'=m.account_id::text
AND r.document->'members'->($2::uuid::text)->>'profile_id'=m.profile_id::text
AND r.document->'members'->($2::uuid::text)->>'state'=m.state
AND r.document->'members'->($2::uuid::text)->>'media_identity'=m.media_identity`, ids.sourceAccount, ids.sourceProfile).Scan(
		&b.RoomID, &b.RoomOperationID, &b.AccountID, &b.ProfileID, &b.Generation, &b.FenceOperationID, &b.MediaIdentity)
	if err != nil {
		return snap, ErrConflict
	}
	snap.Source = b
	return snap, nil
}
