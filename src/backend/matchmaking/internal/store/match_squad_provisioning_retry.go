package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MatchSquadProvisioningClaim struct {
	MatchID      uuid.UUID
	OperationID  uuid.UUID
	Generation   int64
	AttemptCount int
}

type MatchSquadProvisioningAttention struct {
	Stalled          int
	ContractMismatch int
}

func (s *MatchStore) MatchSquadProvisioningAttention(ctx context.Context) (MatchSquadProvisioningAttention, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadProvisioningAttention{}, ErrSourcePartyUnavailable
	}
	var result MatchSquadProvisioningAttention
	err := s.Pool.QueryRow(ctx, `SELECT
		count(*) FILTER(WHERE provision_last_progress_at<=clock_timestamp()-interval '15 minutes'),
		count(*) FILTER(WHERE provision_last_error_class IN ('provider_contract','invalid_argument','failed_precondition'))
		FROM matchmaking_match_squad_operations WHERE state IN ('provisioning','compensating')`).Scan(&result.Stalled, &result.ContractMismatch)
	return result, err
}

func (s *MatchStore) ClaimMatchSquadProvisioningOperation(ctx context.Context, matchID, operationID uuid.UUID, lease time.Duration) (MatchSquadProvisioningClaim, bool, error) {
	if s == nil || s.Pool == nil || matchID == uuid.Nil || operationID == uuid.Nil {
		return MatchSquadProvisioningClaim{}, false, ErrMatchSquadConflict
	}
	if lease < time.Second || lease > 2*time.Minute {
		return MatchSquadProvisioningClaim{}, false, ErrMatchSquadConflict
	}
	claim := MatchSquadProvisioningClaim{MatchID: matchID, OperationID: operationID}
	err := s.Pool.QueryRow(ctx, `UPDATE matchmaking_match_squad_operations SET
		provision_claim_generation=provision_claim_generation+1,provision_attempt_count=provision_attempt_count+1,
		provision_claim_until=clock_timestamp()+$3::BIGINT*interval '1 millisecond'
		WHERE match_id=$1 AND operation_id=$2 AND state IN ('provisioning','compensating')
		AND provision_next_attempt_at<=clock_timestamp()
		AND (provision_claim_until IS NULL OR provision_claim_until<=clock_timestamp())
		RETURNING provision_claim_generation,provision_attempt_count`, matchID, operationID, lease.Milliseconds()).Scan(&claim.Generation, &claim.AttemptCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadProvisioningClaim{}, false, nil
	}
	if err != nil {
		return MatchSquadProvisioningClaim{}, false, err
	}
	return claim, true, nil
}

func (s *MatchStore) ScheduleMatchSquadProvisioningRetry(ctx context.Context, claim MatchSquadProvisioningClaim, delay time.Duration, errorClass string) error {
	if s == nil || s.Pool == nil || claim.MatchID == uuid.Nil || claim.OperationID == uuid.Nil || claim.Generation <= 0 || !validProvisionErrorClass(errorClass) {
		return ErrMatchSquadConflict
	}
	if delay < time.Second {
		delay = time.Second
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET
		provision_claim_until=NULL,provision_next_attempt_at=clock_timestamp()+$4::BIGINT*interval '1 millisecond',provision_last_error_class=$5
		WHERE match_id=$1 AND operation_id=$2 AND provision_claim_generation=$3 AND provision_claim_until>clock_timestamp()
		AND state IN ('provisioning','compensating')`, claim.MatchID, claim.OperationID, claim.Generation, delay.Milliseconds(), errorClass)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMatchSquadConflict
	}
	return nil
}

func validProvisionErrorClass(value string) bool {
	switch value {
	case "waiting_authority", "provider_contract", "unavailable", "unauthenticated", "permission_denied", "deadline_exceeded", "invalid_argument", "failed_precondition", "aborted", "internal", "unknown", "settling":
		return true
	default:
		return false
	}
}

func (s *MatchStore) ReleaseMatchSquadProvisioningClaim(ctx context.Context, claim MatchSquadProvisioningClaim) error {
	if s == nil || s.Pool == nil || claim.MatchID == uuid.Nil || claim.OperationID == uuid.Nil || claim.Generation <= 0 {
		return ErrMatchSquadConflict
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_claim_until=NULL,provision_last_error_class=NULL
		WHERE match_id=$1 AND operation_id=$2 AND provision_claim_generation=$3 AND provision_claim_until IS NOT NULL`, claim.MatchID, claim.OperationID, claim.Generation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMatchSquadConflict
	}
	return nil
}
