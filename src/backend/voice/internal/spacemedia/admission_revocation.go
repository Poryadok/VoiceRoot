package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/spacebarrier"
	"voice/backend/voice/internal/store"
)

func lockRevocationIdentities(ctx context.Context, tx pgx.Tx, a store.SpaceMediaAdmission) error {
	return spacebarrier.LockIdentities(ctx, tx, a.AccountID.String(), a.ProfileID.String())
}

func readRevocationTarget(ctx context.Context, tx pgx.Tx, op uuid.UUID, generation string, target *store.SpaceAdmissionRevocation) error {
	a := &target.Admission
	if err := tx.QueryRow(ctx, `SELECT owner_epoch FROM voice_space_media_admission_owners WHERE operation_id=$1 FOR UPDATE`, op).Scan(&target.OwnerEpoch); err != nil {
		return err
	}
	return tx.QueryRow(ctx, `SELECT operation_id,generation,account_id,profile_id,space_id,room_id,voice_room_id,room_generation,livekit_identity FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 FOR UPDATE`, op, generation).Scan(&a.OperationID, &a.Generation, &a.AccountID, &a.ProfileID, &a.SpaceID, &a.RoomID, &a.VoiceRoomID, &a.RoomGeneration, &a.Identity)
}

// Callers already hold Space, head, and sorted identity locks. The epoch is
// checked before idempotent success too: an old cleanup cannot adopt a takeover.
func requireRevocationTarget(ctx context.Context, tx pgx.Tx, target store.SpaceAdmissionRevocation, fenced bool) error {
	a := target.Admission
	var currentEpoch, fenceEpoch int64
	if err := tx.QueryRow(ctx, `SELECT owner_epoch,dispatch_fenced_epoch FROM voice_space_media_admission_owners WHERE operation_id=$1 FOR UPDATE`, a.OperationID).Scan(&currentEpoch, &fenceEpoch); err != nil {
		return err
	}
	if target.OwnerEpoch <= 0 || currentEpoch != target.OwnerEpoch || (fenced && fenceEpoch != currentEpoch) {
		return ErrAdmissionConflict
	}
	var exact bool
	err := tx.QueryRow(ctx, `SELECT account_id=$3 AND profile_id=$4 AND space_id=$5 AND room_id=$6 AND voice_room_id=$7 AND room_generation=$8 AND livekit_identity=$9 AND participant_state IN ('REVOKING','ENDED') FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 FOR UPDATE`, a.OperationID, a.Generation, a.AccountID, a.ProfileID, a.SpaceID, a.RoomID, a.VoiceRoomID, a.RoomGeneration, a.Identity).Scan(&exact)
	if err != nil {
		return err
	}
	if !exact {
		return ErrAdmissionConflict
	}
	return nil
}

func (s *PostgresAdmissionStore) readRestrictiveAdmissionDispatch(ctx context.Context, target store.SpaceAdmissionRevocation) (int64, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	a := target.Admission
	if err := lockAdmissionCleanup(ctx, tx, a.OperationID, a.Generation); err != nil {
		return 0, false, err
	}
	var head string
	if err := tx.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, a.VoiceRoomID, a.RoomGeneration).Scan(&head); err != nil {
		return 0, false, err
	}
	if err := lockRevocationIdentities(ctx, tx, a); err != nil {
		return 0, false, err
	}
	if err := requireRevocationTarget(ctx, tx, target, false); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, err
	}
	return target.OwnerEpoch, false, nil
}

func (s *PostgresAdmissionStore) FenceRestrictiveRoomLeave(ctx context.Context, target store.SpaceAdmissionRevocation, gates store.SpaceAdmissionProjectionGates) (store.SpaceProjectionTicket, error) {
	if gates == nil {
		return store.SpaceProjectionTicket{}, ErrAdmissionUnavailable
	}
	a := target.Admission
	// Reject stale/incorrect authority before even allocating RESTORING. The
	// loader below still rechecks after the observed gate for CAS freshness.
	if _, _, err := s.readRestrictiveAdmissionDispatch(ctx, target); err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	ticket, err := gates.EnsureAdmissionDispatch(ctx, a.OperationID.String(), func(ctx context.Context, op string) (int64, bool, error) {
		if op != a.OperationID.String() {
			return 0, false, ErrAdmissionConflict
		}
		return s.readRestrictiveAdmissionDispatch(ctx, target)
	})
	if err != nil {
		return ticket, err
	}
	if ticket.AdmissionLive || ticket.AdmissionIncarnation == "" || ticket.AdmissionOwnerEpoch != target.OwnerEpoch {
		return ticket, ErrAdmissionConflict
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ticket, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAdmissionCleanup(ctx, tx, a.OperationID, a.Generation); err != nil {
		return ticket, err
	}
	var head string
	if err := tx.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, a.VoiceRoomID, a.RoomGeneration).Scan(&head); err != nil {
		return ticket, err
	}
	if err := lockRevocationIdentities(ctx, tx, a); err != nil {
		return ticket, err
	}
	if err := requireRevocationTarget(ctx, tx, target, false); err != nil {
		return ticket, err
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_space_media_admission_owners SET dispatch_fenced_epoch=owner_epoch,updated_at=clock_timestamp() WHERE operation_id=$1`, a.OperationID); err != nil {
		return ticket, err
	}
	return ticket, tx.Commit(ctx)
}

func (s *PostgresAdmissionStore) FenceOwnedRoomLeave(ctx context.Context, owner store.SpaceAdmissionOwnership, operation uuid.UUID, gates store.SpaceAdmissionProjectionGates) (store.SpaceProjectionTicket, error) {
	if owner.OperationID != operation || gates == nil {
		return store.SpaceProjectionTicket{}, ErrAdmissionConflict
	}
	ctx = store.WithSpaceAdmissionOwnership(ctx, owner)
	load := func(ctx context.Context, op string) (int64, bool, error) {
		epoch, live, err := s.readAdmissionDispatch(ctx, op, true)
		if err != nil {
			return 0, false, err
		}
		if live {
			return 0, false, ErrAdmissionConflict
		}
		return epoch, false, nil
	}
	if _, _, err := load(ctx, operation.String()); err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	ticket, err := gates.EnsureAdmissionDispatch(ctx, operation.String(), load)
	if err != nil {
		return ticket, err
	}
	if ticket.AdmissionLive || ticket.AdmissionIncarnation == "" || ticket.AdmissionOwnerEpoch != owner.Epoch {
		return ticket, ErrAdmissionConflict
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ticket, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := requireAdmissionOwnership(ctx, tx, operation); err != nil {
		return ticket, err
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_space_media_admission_owners o SET dispatch_fenced_epoch=owner_epoch,updated_at=clock_timestamp() WHERE operation_id=$1 AND owner_token=$2 AND owner_epoch=$3 AND lease_until>clock_timestamp() AND EXISTS(SELECT 1 FROM voice_space_media_admissions a WHERE a.operation_id=o.operation_id AND (a.state='ABORTING' OR a.participant_state IN ('REVOKING','ENDED')))`, operation, owner.Token, owner.Epoch)
	if err != nil {
		return ticket, err
	}
	if tag.RowsAffected() != 1 {
		return ticket, ErrAdmissionConflict
	}
	return ticket, tx.Commit(ctx)
}
