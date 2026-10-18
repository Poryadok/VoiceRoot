package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ValidationFact is submission and owner provenance, never a threshold decision.
// A missing reporter account is legacy/unverified evidence and remains manual-only.
type ValidationFact struct {
	ReportID          uuid.UUID
	ReporterProfileID uuid.UUID
	ReporterAccountID *uuid.UUID
	TargetType        string
	TargetID          uuid.UUID
	TargetAccountID   *uuid.UUID
	CreatedAt         time.Time
}

// ReportValidatedHook may enqueue independent threshold work in this transaction.
// It must not perform remote calls or treat account mapping as subject/revision proof.
type ReportValidatedHook interface {
	ReportValidated(context.Context, pgx.Tx, ValidationFact) error
}

type ValidationClaim struct {
	ValidationFact
	ClaimID           uuid.UUID
	Attempts          int
	LegacyPublication bool
	Category          string
}

// ClaimValidation leases durable work. Expired claims can be recovered after restart.
func (s *ReportStore) ClaimValidation(ctx context.Context) (*ValidationClaim, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	claim := &ValidationClaim{ClaimID: uuid.New()}
	err := s.Pool.QueryRow(ctx, `
WITH ready AS (
 SELECT id FROM reports WHERE validation_state='PENDING'
 AND validation_next_at<=now() AND (validation_claim_until IS NULL OR validation_claim_until<=now())
 ORDER BY validation_next_at,created_at,id FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE reports r SET validation_claim=$1,validation_claim_until=now()+interval '30 seconds',
 validation_attempts=CASE WHEN validation_attempts<2147483647 THEN validation_attempts+1 ELSE validation_attempts END
FROM ready WHERE r.id=ready.id
RETURNING r.id,r.reporter_profile_id,r.reporter_account_id,r.target_type,r.target_id,
 r.created_at,r.validation_attempts,r.validation_publication_legacy,r.category`, claim.ClaimID).Scan(
		&claim.ReportID, &claim.ReporterProfileID, &claim.ReporterAccountID, &claim.TargetType, &claim.TargetID,
		&claim.CreatedAt, &claim.Attempts, &claim.LegacyPublication, &claim.Category)
	if err != nil {
		return nil, err
	}
	return claim, nil
}

// RetryValidation never turns an outage or malformed authority into a deletion.
func (s *ReportStore) RetryValidation(ctx context.Context, claim *ValidationClaim) error {
	if s == nil || s.Pool == nil {
		return errStoreNotConfigured
	}
	if claim == nil {
		return errors.New("missing validation claim")
	}
	delay := time.Second
	for i := 1; i < claim.Attempts && delay < 5*time.Minute; i++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	_, err := s.Pool.Exec(ctx, `UPDATE reports SET validation_claim=NULL,validation_claim_until=NULL,
 validation_next_at=now()+interval '1 second'*$3,validation_failure_code='OWNER_UNAVAILABLE'
 WHERE id=$1 AND validation_state='PENDING' AND validation_claim=$2`, claim.ReportID, claim.ClaimID, delay.Seconds())
	return err
}

// IntakeEvent is an already serialized immutable event, persisted with activation.
type IntakeEvent struct {
	ID            uuid.UUID
	Kind, Subject string
	Payload       []byte
}

// CompleteValidation atomically admits a report to the staff queue and its events.
// A deleted row or superseded claim cannot recreate work or publish anything.
func (s *ReportStore) CompleteValidation(ctx context.Context, claim *ValidationClaim, state string, accountID uuid.UUID, events []IntakeEvent, hook ReportValidatedHook) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errStoreNotConfigured
	}
	if claim == nil {
		return false, errors.New("missing validation claim")
	}
	if state != "LIVE" && state != "MISSING" && state != "DELETED" {
		return false, errors.New("invalid target decision")
	}
	if state != "LIVE" && accountID != uuid.Nil {
		return false, errors.New("unexpected terminal account binding")
	}
	if state == "LIVE" && ((claim.TargetType == "user" && accountID == uuid.Nil) || (claim.TargetType != "user" && accountID != uuid.Nil)) {
		return false, errors.New("invalid target account binding")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var fact ValidationFact
	var legacy bool
	err = tx.QueryRow(ctx, `SELECT id,reporter_profile_id,reporter_account_id,target_type,target_id,created_at,validation_publication_legacy
 FROM reports WHERE id=$1 AND validation_state='PENDING' AND validation_claim=$2
 AND validation_claim_until>now() FOR UPDATE`, claim.ReportID, claim.ClaimID).Scan(
		&fact.ReportID, &fact.ReporterProfileID, &fact.ReporterAccountID, &fact.TargetType, &fact.TargetID, &fact.CreatedAt, &legacy)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if fact.TargetType != claim.TargetType || fact.TargetID != claim.TargetID {
		return false, errors.New("target claim mismatch")
	}
	if state == "LIVE" && !legacy {
		kinds := map[string]bool{}
		for _, event := range events {
			kinds[event.Kind] = true
		}
		if len(events) != 2 || !kinds["domain.report_created"] || !kinds["analytics.report_created"] {
			return false, errors.New("complete report publication required")
		}
	}
	if state != "LIVE" {
		_, err = tx.Exec(ctx, `UPDATE reports SET validation_state=$2,status='dismissed',description=NULL,evidence='{}'::jsonb,
 resolution=jsonb_build_object('code',$3::text),resolved_at=now(),updated_at=now(),validation_claim=NULL,
 validation_claim_until=NULL,validation_failure_code=$3 WHERE id=$1`, fact.ReportID, state, "TARGET_"+map[string]string{"MISSING": "NOT_FOUND", "DELETED": "DELETED"}[state])
	} else {
		if accountID != uuid.Nil {
			fact.TargetAccountID = &accountID
		}
		_, err = tx.Exec(ctx, `UPDATE reports SET validation_state='VALID',validated_target_account_id=$2,
 validation_claim=NULL,validation_claim_until=NULL,validation_failure_code=NULL,updated_at=now() WHERE id=$1`, fact.ReportID, fact.TargetAccountID)
		if err == nil && hook != nil {
			err = hook.ReportValidated(ctx, tx, fact)
		}
		if err == nil && !legacy {
			for _, event := range events {
				if event.ID == uuid.Nil || event.Kind == "" || event.Subject == "" || len(event.Payload) == 0 {
					return false, errors.New("invalid intake event")
				}
				_, err = tx.Exec(ctx, `INSERT INTO report_intake_outbox(id,report_id,kind,subject,payload) VALUES($1,$2,$3,$4,$5)`, event.ID, fact.ReportID, event.Kind, event.Subject, event.Payload)
				if err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
