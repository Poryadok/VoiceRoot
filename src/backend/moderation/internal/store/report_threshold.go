package store

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ReportAudienceMetric = "POTENTIAL_PUBLIC_PROFILE_ACCOUNTS_V1"

type ReportThresholdStore struct {
	Pool      *pgxpool.Pool
	Sanctions *SanctionStore
	// Supplied by BE-045's shared subject authority. It may bind a fresh protected
	// Auth LIVE subject, but cannot invent one from a profile or report UUID.
	BindLiveSubjectTx func(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, uint64) error
}

type ReportThresholdClaim struct {
	ReportID, ClaimID, ProfileID, AccountID uuid.UUID
	CreatedAt                               time.Time
	Attempts                                int
}

// ReportThresholdObservation combines fresh User context and BE-045 Auth subject authority.
// Fetch both outside the account transaction, then validate the complete vector here.
type ReportThresholdObservation struct {
	EvaluationID, ProfileID, AccountID, SubjectID, SnapshotID uuid.UUID
	ProfileRevision, LifecycleRevision                        uint64
	Audience                                                  int64
	Metric                                                    string
	StartedAt, CompletedAt, ExpiresAt                         time.Time
}

// EnqueueTx is called only by the BE-137 VALID hook. SQL verifies its local
// provenance again so replay and legacy rows cannot create automatic authority.
func (s *ReportThresholdStore) EnqueueTx(ctx context.Context, tx pgx.Tx, reportID uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO report_threshold_evaluations(report_id)
SELECT id FROM reports WHERE id=$1 AND target_type='user' AND validation_state='VALID'
AND status IN ('pending','reviewing') AND reporter_account_id IS NOT NULL
AND validated_target_account_id IS NOT NULL AND reporter_account_id<>validated_target_account_id
ON CONFLICT(report_id) DO NOTHING`, reportID)
	return err
}

func (s *ReportThresholdStore) Claim(ctx context.Context) (*ReportThresholdClaim, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	out := &ReportThresholdClaim{ClaimID: uuid.New()}
	err := s.Pool.QueryRow(ctx, `WITH ready AS (
SELECT e.report_id FROM report_threshold_evaluations e
WHERE e.completed_at IS NULL AND e.next_at<=clock_timestamp()
AND (e.claim_until IS NULL OR e.claim_until<=clock_timestamp())
ORDER BY e.next_at,e.report_id FOR UPDATE SKIP LOCKED LIMIT 1
), claimed AS (
UPDATE report_threshold_evaluations e SET claim_id=$1,claim_until=clock_timestamp()+interval '30 seconds',
attempts=LEAST(e.attempts::bigint+1,2147483647)::integer
FROM ready WHERE e.report_id=ready.report_id RETURNING e.report_id,e.attempts)
SELECT r.id,r.target_id,COALESCE(r.validated_target_account_id,'00000000-0000-0000-0000-000000000000'::uuid),r.created_at,c.attempts
FROM claimed c JOIN reports r ON r.id=c.report_id`, out.ClaimID).Scan(&out.ReportID, &out.ProfileID, &out.AccountID, &out.CreatedAt, &out.Attempts)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ReportThresholdStore) Retry(ctx context.Context, claim *ReportThresholdClaim) error {
	if s == nil || s.Pool == nil || claim == nil {
		return errStoreNotConfigured
	}
	delay := time.Second
	for attempt := 1; attempt < claim.Attempts && delay < 5*time.Minute; attempt++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	_, err := s.Pool.Exec(ctx, `UPDATE report_threshold_evaluations SET claim_id=NULL,claim_until=NULL,
next_at=clock_timestamp()+interval '1 second'*$3,failure_code='AUTHORITY_UNAVAILABLE'
WHERE report_id=$1 AND claim_id=$2 AND completed_at IS NULL`, claim.ReportID, claim.ClaimID, delay.Seconds())
	return err
}

// AfterSanctionClosedTx runs in the sanction owner's account-serialized transaction.
// Closing a shadow ban prevents any pre-close account evidence immediately rebanning it.
func (s *ReportThresholdStore) AfterSanctionClosedTx(ctx context.Context, tx pgx.Tx, sanction *SanctionRow) error {
	if sanction == nil || sanction.Type != "shadow_ban" {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO report_threshold_account_cutoffs(account_id,closed_at) VALUES($1,clock_timestamp())
ON CONFLICT(account_id) DO UPDATE SET closed_at=GREATEST(report_threshold_account_cutoffs.closed_at,EXCLUDED.closed_at)`, sanction.TargetAccountID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE report_threshold_cases SET closed_at=clock_timestamp()
WHERE account_id=$1 AND closed_at IS NULL AND id IN (SELECT machine_case_id FROM sanctions WHERE id=$2)`, sanction.TargetAccountID, sanction.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE report_threshold_outbox o SET cancelled_at=clock_timestamp(),failure_code='CASE_CLOSED'
FROM report_threshold_cases c WHERE o.case_id=c.id AND c.account_id=$1 AND c.closed_at IS NOT NULL
AND o.delivered_at IS NULL AND o.cancelled_at IS NULL`, sanction.TargetAccountID)
	return err
}

func (o ReportThresholdObservation) valid(claim *ReportThresholdClaim, now time.Time) bool {
	return claim != nil && o.EvaluationID == claim.ClaimID && o.ProfileID == claim.ProfileID && o.AccountID == claim.AccountID &&
		o.SubjectID != uuid.Nil && o.SnapshotID != uuid.Nil && o.ProfileRevision > 0 && o.ProfileRevision <= math.MaxInt64 &&
		o.LifecycleRevision > 0 && o.LifecycleRevision <= math.MaxInt64 && o.Audience >= 0 && o.Metric == ReportAudienceMetric &&
		!o.StartedAt.IsZero() && !o.CompletedAt.Before(o.StartedAt) && !o.CompletedAt.After(now) &&
		o.ExpiresAt.After(now) && !o.ExpiresAt.After(o.StartedAt.Add(30*time.Second)) && now.Sub(o.StartedAt) < 30*time.Second
}

var errReportThresholdStale = errors.New("report threshold observation or claim stale")

// FinishManual retains the accepted receipt and never invents automatic authority.
func (s *ReportThresholdStore) FinishManual(ctx context.Context, claim *ReportThresholdClaim, outcome string) error {
	if s == nil || s.Pool == nil || claim == nil {
		return errStoreNotConfigured
	}
	switch outcome {
	case "INELIGIBLE_EVIDENCE", "EMPTY_AUDIENCE", "TARGET_MISSING", "TARGET_DELETED":
	default:
		return errReportThresholdStale
	}
	_, err := s.Pool.Exec(ctx, `UPDATE report_threshold_evaluations SET completed_at=clock_timestamp(),outcome=$3,
claim_id=NULL,claim_until=NULL,failure_code=NULL WHERE report_id=$1 AND claim_id=$2 AND completed_at IS NULL
AND claim_until>clock_timestamp()`, claim.ReportID, claim.ClaimID, outcome)
	return err
}
