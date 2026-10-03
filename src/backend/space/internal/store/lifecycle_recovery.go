package store

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

func (s *SpaceStore) DecideExpiredLifecyclePurge(ctx context.Context, id uuid.UUID) (*spacecore.LifecycleAggregate, error) {
	return s.withLockedLifecycle(ctx, id, func(tx pgx.Tx, a *spacecore.LifecycleAggregate) (bool, error) {
		if a.Phase() != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED {
			return false, nil
		}
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return false, err
		}
		if now.Before(a.PurgeAfter()) {
			return false, nil
		}
		if a.Generation() >= math.MaxInt64 {
			return false, ErrLifecycleEvidenceInvalid
		}
		_, err := a.DecideRecovery(now.UTC(), a.Generation()+1)
		return err == nil, err
	})
}

type LifecycleAttempt struct {
	SpaceID    uuid.UUID
	Generation uint64
	Phase      string
	Stalled    bool
}

// ListDueLifecycleAttempts uses database time. Waiting for the seven-day
// deadline is healthy and does not trigger a no-progress alert.
func (s *SpaceStore) ListDueLifecycleAttempts(ctx context.Context, limit int) ([]LifecycleAttempt, error) {
	if s == nil || s.Pool == nil || limit < 1 || limit > 100 {
		return nil, ErrLifecycleEvidenceInvalid
	}
	rows, err := s.Pool.Query(ctx, `SELECT space_id,generation,phase,
  phase<>'SCHEDULED' AND progress_at<=clock_timestamp()-interval '15 minutes'
  FROM space_lifecycle_aggregates WHERE phase NOT IN ('LIVE','PURGED')
  AND next_attempt_at<=clock_timestamp() AND (phase<>'SCHEDULED' OR purge_after<=clock_timestamp())
  ORDER BY next_attempt_at,space_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []LifecycleAttempt
	for rows.Next() {
		var a LifecycleAttempt
		if err := rows.Scan(&a.SpaceID, &a.Generation, &a.Phase, &a.Stalled); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// DeferLifecycleAttempt persists unbounded retries: 1s exponential base to a
// 5m total cap, with up to 20% positive jitter. New progress resets the exponent.
// It stores no downstream error bodies or credentials.
func (s *SpaceStore) DeferLifecycleAttempt(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.Pool == nil || id == uuid.Nil {
		return ErrLifecycleEvidenceInvalid
	}
	_, err := s.Pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET
  next_attempt_at=clock_timestamp()+make_interval(secs=>LEAST(300.0,power(2.0,LEAST(retry_attempt,9))*(1.0+random()*0.2))),
  retry_attempt=LEAST(retry_attempt+1,1000000),last_failure_at=clock_timestamp()
  WHERE space_id=$1 AND phase NOT IN ('LIVE','PURGED')`, id)
	return err
}

// LifecycleProofLookup reconstructs only digest-bound receipt lookup inputs.
// Background recovery never stores or resubmits the plaintext proof/name.
func (s *SpaceStore) LifecycleProofLookup(ctx context.Context, id uuid.UUID) (*authv1.GetSpaceDeletionProofReceiptRequest, error) {
	a, err := s.LoadLifecycle(ctx, id)
	if err != nil {
		return nil, err
	}
	op, err := loadLifecycleOperation(ctx, s.db(), uuid.MustParse(a.Snapshot().DeletionOperationID))
	if err != nil {
		return nil, err
	}
	if err := op.validate(); err != nil {
		return nil, err
	}
	if op.method != "DELETE" || op.spaceID != id {
		return nil, ErrLifecycleConflict
	}
	return &authv1.GetSpaceDeletionProofReceiptRequest{ProtocolVersion: 1, AccountId: op.accountID.String(), ProfileId: op.actorProfileID.String(), SessionEpoch: op.sessionEpoch, SpaceId: id.String(), OperationId: op.operationID.String(), ConfirmationNameSha256: op.confirmationHash, ProofDigestSha256: op.proofDigest}, nil
}
