package spacemedia

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/spacebarrier"
	"voice/backend/voice/internal/spacelifecycle"
	"voice/backend/voice/internal/store"
)

// ResumeCapturedDrain never discovers a newer incarnation by Space/profile.
// Ejection has already used the exact saved SFU target, even if Redis is absent.
func (c *Coordinator) ResumeCapturedDrain(ctx context.Context, t spacelifecycle.CapturedTarget) error {
	if c == nil || c.Calls == nil || c.Admissions == nil {
		return ErrAdmissionUnavailable
	}
	call, err := c.Calls.GetCall(ctx, t.RoomID)
	if err == nil {
		if p, ok := call.SpaceMedia[t.ProfileID]; ok {
			if p.AdmissionOperationID != t.OperationID || p.Generation != t.Generation || p.Identity != t.Identity || p.RoomGeneration != t.RoomGeneration {
				return ErrAdmissionConflict
			}
			_, matched, err := c.Store.BeginSpaceMediaRevocation(ctx, t.RoomID, t.ProfileID, t.Identity, t.Generation)
			if err != nil || !matched {
				if err != nil {
					return err
				}
				return ErrAdmissionConflict
			}
			_, matched, err = c.Store.CompleteSpaceMediaRevocation(ctx, t.RoomID, t.ProfileID, t.Identity, t.Generation)
			if err != nil || !matched {
				if err != nil {
					return err
				}
				return ErrAdmissionConflict
			}
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	cleanup, ok := c.Admissions.(interface {
		CompleteCapturedDrain(context.Context, spacelifecycle.CapturedTarget) error
	})
	if !ok {
		return ErrAdmissionUnavailable
	}
	return cleanup.CompleteCapturedDrain(ctx, t)
}

func (s *PostgresAdmissionStore) CompleteCapturedDrain(ctx context.Context, t spacelifecycle.CapturedTarget) error {
	op, err := uuid.Parse(t.OperationID)
	if err != nil {
		return ErrAdmissionConflict
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAdmissionCleanup(ctx, tx, op, t.Generation); err != nil {
		return err
	}
	var state string
	if err := tx.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, t.VoiceRoomID, int64(t.RoomGeneration)).Scan(&state); err != nil {
		return err
	}
	if err := spacebarrier.LockIdentities(ctx, tx, t.AccountID, t.ProfileID); err != nil {
		return err
	}
	var ended bool
	if err := tx.QueryRow(ctx, `SELECT participant_state='ENDED' AND (state='COMMITTED' OR cleanup_completed_at IS NOT NULL) FROM voice_space_media_admissions
WHERE operation_id=$1 AND generation=$2 AND account_id=$3 AND profile_id=$4 AND space_id=$5 AND room_id=$6 AND voice_room_id=$7 AND room_generation=$8 AND livekit_identity=$9 FOR UPDATE`, op, t.Generation, t.AccountID, t.ProfileID, t.SpaceID, t.RoomID, t.VoiceRoomID, int64(t.RoomGeneration), t.Identity).Scan(&ended); err != nil {
		return err
	}
	if ended {
		return tx.Commit(ctx)
	}
	var fenceOp *uuid.UUID
	var fenceGeneration *string
	var fenceProfile, room string
	err = tx.QueryRow(ctx, `SELECT admission_operation_id,admission_generation,profile_id,room_id FROM voice_account_voice_fences WHERE account_id=$1 FOR UPDATE`, t.AccountID).Scan(&fenceOp, &fenceGeneration, &fenceProfile, &room)
	if err == nil {
		if fenceOp == nil || fenceGeneration == nil || *fenceOp != op || *fenceGeneration != t.Generation || fenceProfile != t.ProfileID || room != t.RoomID {
			return ErrAdmissionConflict
		}
		if _, err := tx.Exec(ctx, `DELETE FROM voice_account_voice_fences WHERE account_id=$1 AND admission_operation_id=$2 AND admission_generation=$3`, t.AccountID, op, t.Generation); err != nil {
			return err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	} else if t.State != "PREPARED" {
		return ErrAdmissionConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_space_media_admissions SET participant_state='ENDED',cleanup_completed_at=CASE WHEN state='ABORTING' THEN COALESCE(cleanup_completed_at,clock_timestamp()) ELSE cleanup_completed_at END,updated_at=clock_timestamp() WHERE operation_id=$1 AND generation=$2`, op, t.Generation); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
