package spacemedia

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/spacebarrier"
	"voice/backend/voice/internal/store"
)

// The lease covers multiple bounded authority/Redis operations, rather than an
// updated_at heuristic. Renew at the coordinator's 5s cadence, with six renewal
// opportunities in the 30s lease. Every SQL mutation still checks DB time.
const admissionOwnershipLease = 30 * time.Second

func (s *PostgresAdmissionStore) closeExpiredOrphanHead(ctx context.Context, h store.SpaceMediaRoomHead) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := spacebarrier.Lock(ctx, tx, h.SpaceID); err != nil {
		return err
	}
	var state string
	var creator uuid.UUID
	err = tx.QueryRow(ctx, `SELECT state,creator_operation_id FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, h.VoiceRoomID, int64(h.RoomGeneration)).Scan(&state, &creator)
	if err != nil {
		return err
	}
	if state != "OPEN" || creator != h.CreatorOperationID {
		return nil
	}
	var expired bool
	var epoch int64
	err = tx.QueryRow(ctx, `SELECT lease_until<=clock_timestamp(),owner_epoch FROM voice_space_media_admission_owners WHERE operation_id=$1 FOR UPDATE`, creator).Scan(&expired, &epoch)
	if err != nil {
		return err
	}
	if !expired {
		return nil
	}
	if epoch == 9223372036854775807 {
		return ErrAdmissionConflict
	}
	var hasAdmission bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_space_media_admissions WHERE operation_id=$1 OR (voice_room_id=$2 AND room_generation=$3))`, creator, h.VoiceRoomID, int64(h.RoomGeneration)).Scan(&hasAdmission)
	if err != nil {
		return err
	}
	if hasAdmission {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_space_media_admission_owners SET owner_token=$2,owner_epoch=owner_epoch+1,lease_until=clock_timestamp(),updated_at=clock_timestamp() WHERE operation_id=$1`, creator, uuid.New()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_space_media_room_heads SET state='ENDED',updated_at=clock_timestamp() WHERE voice_room_id=$1 AND room_generation=$2 AND creator_operation_id=$3 AND state='OPEN'`, h.VoiceRoomID, int64(h.RoomGeneration), creator); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func requireAdmissionOwnership(ctx context.Context, tx pgx.Tx, operation uuid.UUID) error {
	owner, ok := store.SpaceAdmissionOwnershipFromContext(ctx, operation)
	if !ok {
		return ErrAdmissionConflict
	}
	var current bool
	err := tx.QueryRow(ctx, `SELECT owner_token=$2 AND owner_epoch=$3 AND lease_until>clock_timestamp()
FROM voice_space_media_admission_owners WHERE operation_id=$1 FOR UPDATE`, operation, owner.Token, owner.Epoch).Scan(&current)
	if err != nil || !current {
		return ErrAdmissionConflict
	}
	return nil
}

func (s *PostgresAdmissionStore) RenewAdmissionOwnership(ctx context.Context, operation uuid.UUID) error {
	owner, ok := store.SpaceAdmissionOwnershipFromContext(ctx, operation)
	if !s.usable() || !ok {
		return ErrAdmissionConflict
	}
	tag, err := s.pool.Exec(ctx, `UPDATE voice_space_media_admission_owners
SET lease_until=clock_timestamp()+$4,updated_at=clock_timestamp()
WHERE operation_id=$1 AND owner_token=$2 AND owner_epoch=$3 AND lease_until>clock_timestamp()`, operation, owner.Token, owner.Epoch, admissionOwnershipLease)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	return nil
}

func (s *PostgresAdmissionStore) RelinquishAdmissionOwnership(ctx context.Context, operation uuid.UUID) error {
	owner, ok := store.SpaceAdmissionOwnershipFromContext(ctx, operation)
	if !s.usable() || !ok {
		return ErrAdmissionConflict
	}
	tag, err := s.pool.Exec(ctx, `UPDATE voice_space_media_admission_owners
SET lease_until=LEAST(lease_until,clock_timestamp()),updated_at=clock_timestamp()
WHERE operation_id=$1 AND owner_token=$2 AND owner_epoch=$3`, operation, owner.Token, owner.Epoch)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	return nil
}

// HoldAdmissionOwnership cancels all subsequent forward work after a failed or
// ambiguous renewal. Stop joins the renewer before exact-owner relinquish; a
// failed acknowledgement leaves a bounded durable lease for recovery readback.
func (s *PostgresAdmissionStore) HoldAdmissionOwnership(ctx context.Context, operation uuid.UUID) (context.Context, func(), error) {
	if err := s.RenewAdmissionOwnership(ctx, operation); err != nil {
		return nil, nil, err
	}
	owned, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(ReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-owned.Done():
				return
			case <-ticker.C:
				budget, stop := context.WithTimeout(owned, 2*time.Second)
				err := s.RenewAdmissionOwnership(budget, operation)
				stop()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	return owned, func() {
		cancel()
		<-done
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer stop()
		_ = s.RelinquishAdmissionOwnership(cleanup, operation)
	}, nil
}

// ClaimAdmissionRecovery is a CAS, not authority granted by enumeration. The
// Space barrier and exact room head precede ownership/admission locks; no IO is
// performed while the transaction holds them. Return current state, not a's state.
func (s *PostgresAdmissionStore) ClaimAdmissionRecovery(ctx context.Context, a Admission) (Admission, bool, error) {
	if !s.usable() {
		return Admission{}, false, ErrAdmissionUnavailable
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Admission{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAdmissionCleanup(ctx, tx, a.OperationID, a.Generation); err != nil {
		return Admission{}, false, err
	}
	var voiceRoom string
	var roomGeneration int64
	if err := tx.QueryRow(ctx, `SELECT voice_room_id,room_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, a.OperationID, a.Generation).Scan(&voiceRoom, &roomGeneration); err != nil {
		return Admission{}, false, err
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, voiceRoom, roomGeneration); err != nil {
		return Admission{}, false, err
	}
	owner := store.SpaceAdmissionOwnership{OperationID: a.OperationID, Token: uuid.New()}
	err = tx.QueryRow(ctx, `UPDATE voice_space_media_admission_owners o
SET owner_token=$2,owner_epoch=owner_epoch+1,lease_until=clock_timestamp()+$3,updated_at=clock_timestamp()
WHERE operation_id=$1 AND lease_until<=clock_timestamp() AND owner_epoch<9223372036854775807
 AND NOT EXISTS(SELECT 1 FROM voice_space_lifecycle_targets t WHERE t.source_kind='native' AND t.target_id='native:'||o.operation_id)
RETURNING owner_epoch`, a.OperationID, owner.Token, admissionOwnershipLease).Scan(&owner.Epoch)
	if errors.Is(err, pgx.ErrNoRows) {
		var exhausted bool
		if readErr := tx.QueryRow(ctx, `SELECT owner_epoch=9223372036854775807 AND lease_until<=clock_timestamp() FROM voice_space_media_admission_owners WHERE operation_id=$1`, a.OperationID).Scan(&exhausted); readErr != nil {
			return Admission{}, false, readErr
		}
		if exhausted {
			return Admission{}, false, ErrAdmissionConflict
		}
		return Admission{}, false, nil
	}
	if err != nil {
		return Admission{}, false, err
	}
	var room, session, access, policy int64
	err = tx.QueryRow(ctx, `SELECT operation_id,generation,account_id,profile_id,space_id,room_id,voice_room_id,room_generation,livekit_identity,created_room,call_started_at,max_participants,session_epoch,access_epoch,policy_epoch,can_join,can_publish_audio,can_subscribe,state,participant_state,projection_applied_at IS NOT NULL,lifecycle_generation
FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 AND cleanup_completed_at IS NULL AND (participant_state<>'ENDED' OR state='ABORTING') FOR UPDATE`, a.OperationID, a.Generation).Scan(
		&a.OperationID, &a.Generation, &a.AccountID, &a.ProfileID, &a.SpaceID, &a.RoomID, &a.VoiceRoomID, &room, &a.Identity, &a.CreatedRoom, &a.CallStartedAt, &a.MaxParticipants, &session, &access, &policy, &a.CanJoin, &a.CanPublishAudio, &a.CanSubscribe, &a.State, &a.ParticipantState, &a.ProjectionApplied, &a.LifecycleGeneration)
	if errors.Is(err, pgx.ErrNoRows) {
		return Admission{}, false, nil
	}
	if err != nil {
		return Admission{}, false, err
	}
	if room <= 0 || session <= 0 || access <= 0 || policy <= 0 {
		return Admission{}, false, ErrAdmissionConflict
	}
	a.RoomGeneration, a.SessionEpoch, a.AccessEpoch, a.PolicyEpoch = uint64(room), uint64(session), uint64(access), uint64(policy)
	a.Ownership = owner
	if a.State == AdmissionPrepared || a.State == AdmissionFenced {
		if _, err := tx.Exec(ctx, `UPDATE voice_space_media_admissions SET state='ABORTING',updated_at=clock_timestamp() WHERE operation_id=$1 AND generation=$2 AND state IN ('PREPARED','FENCED')`, a.OperationID, a.Generation); err != nil {
			return Admission{}, false, err
		}
		a.State = AdmissionAborting
	}
	if err := tx.Commit(ctx); err != nil {
		return Admission{}, false, err
	}
	return a, true, nil
}
