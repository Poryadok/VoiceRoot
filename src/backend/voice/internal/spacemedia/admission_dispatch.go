package spacemedia

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/spacebarrier"
	"voice/backend/voice/internal/store"
)

// ReadAdmissionDispatch is fresh maintenance authority sampled after the exact
// Redis gate observation. Settled ACTIVE projection may be repaired/read without
// borrowing workflow authority; every unfinished or aborting row needs its owner.
func (s *PostgresAdmissionStore) ReadAdmissionDispatch(ctx context.Context, operation string) (int64, bool, error) {
	return s.readAdmissionDispatch(ctx, operation, false)
}

func (s *PostgresAdmissionStore) readAdmissionDispatch(ctx context.Context, operation string, owned bool) (int64, bool, error) {
	op, err := uuid.Parse(operation)
	if err != nil || op == uuid.Nil || !s.usable() {
		return 0, false, ErrAdmissionUnavailable
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var generation string
	if err := tx.QueryRow(ctx, `SELECT generation FROM voice_space_media_admissions WHERE operation_id=$1`, op).Scan(&generation); err != nil {
		return 0, false, err
	}
	if err := lockAdmissionCleanup(ctx, tx, op, generation); err != nil {
		return 0, false, err
	}
	var space, account, profile, state, participant, headState string
	var applied bool
	var savedLifecycle int64
	err = tx.QueryRow(ctx, `SELECT a.space_id,a.account_id,a.profile_id,a.state,a.participant_state,a.projection_applied_at IS NOT NULL,h.state,a.lifecycle_generation
FROM voice_space_media_admissions a JOIN voice_space_media_room_heads h USING(voice_room_id,room_generation)
WHERE a.operation_id=$1 AND a.generation=$2 FOR UPDATE OF h`, op, generation).Scan(&space, &account, &profile, &state, &participant, &applied, &headState, &savedLifecycle)
	if err != nil {
		return 0, false, err
	}
	if err := spacebarrier.LockIdentities(ctx, tx, account, profile); err != nil {
		return 0, false, err
	}
	var epoch int64
	if err := tx.QueryRow(ctx, `SELECT owner_epoch FROM voice_space_media_admission_owners WHERE operation_id=$1 FOR UPDATE`, op).Scan(&epoch); err != nil {
		return 0, false, err
	}
	if owned || state != AdmissionCommitted || !applied || participant != "ACTIVE" {
		if err := requireAdmissionOwnership(ctx, tx, op); err != nil {
			return 0, false, err
		}
	}
	live := state == AdmissionCommitted && (participant == "ACTIVE" || participant == "PREPARED") && headState == "OPEN"
	if live {
		current, err := spacebarrier.Live(ctx, tx, space)
		if err != nil {
			return 0, false, err
		}
		live = current == savedLifecycle
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, err
	}
	return epoch, live, nil
}

// FenceAbortedAdmission first establishes a sink-checked DENY and only then
// records the matching epoch acknowledgement. Release/cleanup require it.
// A failed Redis call or unknown acknowledgement leaves cleanup pending.
func (s *PostgresAdmissionStore) FenceAbortedAdmission(ctx context.Context, operation uuid.UUID, gates store.SpaceAdmissionProjectionGates) error {
	owner, ok := store.SpaceAdmissionOwnershipFromContext(ctx, operation)
	if !ok {
		return ErrAdmissionConflict
	}
	_, err := s.FenceOwnedRoomLeave(ctx, owner, operation, gates)
	return err
}

func requireAdmissionDispatchFenced(ctx context.Context, tx pgx.Tx, operation uuid.UUID) error {
	var fenced bool
	if err := tx.QueryRow(ctx, `SELECT dispatch_fenced_epoch=owner_epoch FROM voice_space_media_admission_owners WHERE operation_id=$1`, operation).Scan(&fenced); err != nil {
		return err
	}
	if !fenced {
		return ErrAdmissionConflict
	}
	return nil
}
