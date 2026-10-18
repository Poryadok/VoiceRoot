package spacemedia

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/spacebarrier"
)

// ReserveSpaceGrant authorizes exact saved claims before signing/disclosure.
// grantID is stable through unknown-commit readback. A retry cannot extend exp.
func (s *PostgresAdmissionStore) ReserveSpaceGrant(ctx context.Context, grantID, operation uuid.UUID, generation string, grant livekit.SpaceRoomGrant) (time.Time, time.Time, error) {
	if !s.usable() || grantID == uuid.Nil || operation == uuid.Nil {
		return time.Time{}, time.Time{}, ErrAdmissionUnavailable
	}
	raw, err := json.Marshal(grant)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	digest := sha256.Sum256(raw)
	identity, err := livekit.SpaceParticipantIdentity(grant.ProfileID, grant.MediaGeneration)
	if err != nil {
		return time.Time{}, time.Time{}, ErrAdmissionConflict
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAdmissionLive(ctx, tx, operation, generation); err != nil {
		return time.Time{}, time.Time{}, err
	}
	var room string
	var roomGeneration int64
	if err := tx.QueryRow(ctx, `SELECT voice_room_id,room_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operation, generation).Scan(&room, &roomGeneration); err != nil {
		return time.Time{}, time.Time{}, err
	}
	var open bool
	if err := tx.QueryRow(ctx, `SELECT state='OPEN' FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, room, roomGeneration).Scan(&open); err != nil || !open {
		return time.Time{}, time.Time{}, ErrRoomHeadPending
	}
	if err := spacebarrier.LockIdentities(ctx, tx, grant.AccountID, grant.ProfileID); err != nil {
		return time.Time{}, time.Time{}, err
	}
	var valid bool
	var lifecycle int64
	err = tx.QueryRow(ctx, `SELECT state='COMMITTED' AND participant_state='ACTIVE' AND projection_applied_at IS NOT NULL
AND account_id::text=$3 AND profile_id::text=$4 AND space_id::text=$5 AND voice_room_id::text=$6
AND generation=$7 AND session_epoch=$8 AND access_epoch=$9 AND policy_epoch=$10
AND can_join=$11 AND can_publish_audio=$12 AND can_subscribe=$13 AND livekit_identity=$14,lifecycle_generation
FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 FOR UPDATE`, operation, generation,
		grant.AccountID, grant.ProfileID, grant.SpaceID, grant.VoiceRoomID, grant.MediaGeneration, int64(grant.SessionEpoch), int64(grant.AccessEpoch), int64(grant.PolicyEpoch), grant.CanJoin, grant.CanPublishAudio, grant.CanSubscribe, identity).Scan(&valid, &lifecycle)
	if err != nil || !valid || grant.LiveKitRoomName != "voice-room-"+room {
		return time.Time{}, time.Time{}, ErrAdmissionConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO voice_space_media_grants(grant_id,operation_id,admission_generation,lifecycle_generation,claims_sha256,claims,issued_at,expires_at)
SELECT $1,$2,$3,$4,$5,$6,at,at+interval '60 seconds' FROM (SELECT date_trunc('second',clock_timestamp()) at) t ON CONFLICT(grant_id) DO NOTHING`, grantID, operation, generation, lifecycle, digest[:], raw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	var issued, expires time.Time
	var saved []byte
	var savedOperation uuid.UUID
	var savedGeneration string
	err = tx.QueryRow(ctx, `SELECT issued_at,expires_at,claims_sha256,operation_id,admission_generation FROM voice_space_media_grants WHERE grant_id=$1`, grantID).Scan(&issued, &expires, &saved, &savedOperation, &savedGeneration)
	if err != nil || !equalDigest(saved, digest[:]) || savedOperation != operation || savedGeneration != generation {
		return time.Time{}, time.Time{}, ErrAdmissionConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, time.Time{}, err
	}
	return issued, expires, nil
}
