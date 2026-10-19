package store

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"time"
	"voice/backend/moderation/internal/automod"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ValidationFact is submission and owner provenance, never a threshold decision.
// A missing reporter account is legacy/unverified evidence and remains manual-only.
type ValidationFact struct {
	ReportID                uuid.UUID
	ReporterProfileID       uuid.UUID
	ReporterAccountID       *uuid.UUID
	TargetType              string
	TargetID                uuid.UUID
	TargetAccountID         *uuid.UUID
	CreatedAt               time.Time
	ReporterSubjectID       *uuid.UUID
	ReporterSubjectRevision *int64
	TargetSubjectID         *uuid.UUID
	TargetSubjectRevision   *int64
	ReporterProvenance      string
	TargetProvenance        string
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
	ReporterSubject   *automod.Subject
	TargetSubject     *automod.Subject
	PrivacyCheck      ReportPrivacyCheck
}

// ClaimValidation leases durable work. Expired claims can be recovered after restart.
func (s *ReportStore) ClaimValidation(ctx context.Context) (*ValidationClaim, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	claim := &ValidationClaim{ClaimID: uuid.New()}
	err := s.Pool.QueryRow(ctx, `
WITH ready AS (
 SELECT id FROM reports WHERE validation_state='PENDING' AND reporter_provenance='VERIFIED_AT_SUBMISSION'
 AND validation_next_at<=now() AND (validation_claim_until IS NULL OR validation_claim_until<=now())
 ORDER BY validation_next_at,created_at,id FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE reports r SET validation_claim=$1,validation_claim_until=now()+interval '30 seconds',
 validation_attempts=CASE WHEN validation_attempts<2147483647 THEN validation_attempts+1 ELSE validation_attempts END
FROM ready WHERE r.id=ready.id
RETURNING r.id,r.reporter_profile_id,r.reporter_account_id,r.target_type,r.target_id,
 r.created_at,r.validation_attempts,r.validation_publication_legacy,r.category,r.reporter_subject_id,r.reporter_subject_revision,r.reporter_provenance`, claim.ClaimID).Scan(
		&claim.ReportID, &claim.ReporterProfileID, &claim.ReporterAccountID, &claim.TargetType, &claim.TargetID,
		&claim.CreatedAt, &claim.Attempts, &claim.LegacyPublication, &claim.Category, &claim.ReporterSubjectID, &claim.ReporterSubjectRevision, &claim.ReporterProvenance)
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
	if s.Subjects == nil || claim.PrivacyCheck == nil {
		return false, ErrReportPrivacy
	}
	if claim.ReporterProvenance != "VERIFIED_AT_SUBMISSION" {
		return false, ErrReportPrivacy
	}
	if err := claim.PrivacyCheck(ctx); err != nil {
		return false, err
	}
	if claim.ReporterAccountID == nil || claim.ReporterSubjectID == nil || claim.ReporterSubjectRevision == nil || claim.ReporterSubject == nil || claim.ReporterSubject.AccountID != *claim.ReporterAccountID || claim.ReporterSubject.ID != *claim.ReporterSubjectID || claim.ReporterSubject.Revision < *claim.ReporterSubjectRevision {
		return false, ErrReportPrivacy
	}
	if state == "LIVE" && accountID != uuid.Nil && (claim.TargetSubject == nil || claim.TargetSubject.AccountID != accountID) {
		return false, ErrReportPrivacy
	}
	if claim.TargetSubject != nil && (state != "LIVE" || claim.TargetType != "user" || accountID == uuid.Nil || claim.TargetSubject.AccountID != accountID) {
		return false, ErrReportPrivacy
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
	observed, journal, err := s.readReportValidationObservation(ctx, claim.ReportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	accounts := []uuid.UUID{*claim.ReporterAccountID, accountID}
	for _, account := range []*uuid.UUID{observed.ReporterAccount, observed.TargetAccount} {
		if account != nil {
			accounts = append(accounts, *account)
		}
	}
	if err = lockReportAccounts(ctx, tx, accounts...); err != nil {
		return false, err
	}
	var knownSubjects []reportDispatchBinding
	for _, subject := range []*uuid.UUID{observed.ReporterSubject, observed.TargetSubject} {
		if subject != nil {
			knownSubjects = append(knownSubjects, reportDispatchBinding{subject: *subject})
		}
	}
	for _, record := range journal {
		if record.SubjectID != nil {
			knownSubjects = append(knownSubjects, reportDispatchBinding{subject: *record.SubjectID})
		}
	}
	knownSubjects = append(knownSubjects, reportDispatchBinding{subject: claim.ReporterSubject.ID})
	if claim.TargetSubject != nil {
		knownSubjects = append(knownSubjects, reportDispatchBinding{subject: claim.TargetSubject.ID})
	}
	if err = lockDispatchSubjects(ctx, tx, knownSubjects); err != nil {
		return false, err
	}
	bindings := []automod.Subject{*claim.ReporterSubject}
	if claim.TargetSubject != nil {
		bindings = append(bindings, *claim.TargetSubject)
	}
	if len(bindings) == 2 && bindings[0].ID == bindings[1].ID {
		if bindings[0].AccountID != bindings[1].AccountID || bindings[0].Revision != bindings[1].Revision || bindings[0].State != bindings[1].State || (bindings[0].PurgeAt == nil) != (bindings[1].PurgeAt == nil) || (bindings[0].PurgeAt != nil && !bindings[0].PurgeAt.Equal(*bindings[1].PurgeAt)) {
			return false, ErrReportPrivacy
		}
		bindings = bindings[:1]
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].ID.String() < bindings[j].ID.String() })
	for _, binding := range bindings {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-subject-lifecycle:' || $1::text,0))`, binding.ID.String()); err != nil {
			return false, err
		}
	}
	terminal := false
	var terminalBindings []automod.Subject
	for _, binding := range bindings {
		var ended bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, binding.ID).Scan(&ended); err != nil {
			return false, err
		}
		if ended {
			terminal = true
			terminalBindings = append(terminalBindings, binding)
		}
	}
	// Inspect every permanent fence before any binding. A stale expired Auth
	// capture is retryable, not permission to erase a newer restored identity.
	if !terminal {
		for _, binding := range bindings {
			if err = s.Subjects.BindRetainedSubjectTx(ctx, tx, binding); err != nil {
				return false, errors.Join(ErrReportPrivacy, err)
			}
		}
	}
	checkRetained := func() error {
		for _, binding := range bindings {
			if e := s.Subjects.RequireRetainedSubjectTx(ctx, tx, binding); e != nil {
				return errors.Join(ErrReportPrivacy, e)
			}
		}
		return nil
	}
	var fact ValidationFact
	var legacy bool
	var leaseUntil time.Time
	err = tx.QueryRow(ctx, `SELECT id,reporter_profile_id,reporter_account_id,target_type,target_id,created_at,validation_publication_legacy,reporter_subject_id,reporter_subject_revision,reporter_provenance,validation_claim_until
 FROM reports WHERE id=$1 AND validation_state='PENDING' AND validation_claim=$2
 AND validation_claim_until>clock_timestamp() FOR UPDATE`, claim.ReportID, claim.ClaimID).Scan(
		&fact.ReportID, &fact.ReporterProfileID, &fact.ReporterAccountID, &fact.TargetType, &fact.TargetID, &fact.CreatedAt, &legacy, &fact.ReporterSubjectID, &fact.ReporterSubjectRevision, &fact.ReporterProvenance, &leaseUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	checkLease := func() error {
		var live bool
		if e := tx.QueryRow(ctx, `SELECT $1::timestamptz>clock_timestamp()`, leaseUntil).Scan(&live); e != nil {
			return e
		}
		if !live {
			return ErrReportPrivacy
		}
		return nil
	}
	// The WHERE predicate can run before a row-lock wait. The acquired row
	// therefore needs a separate current database-clock check.
	if err = checkLease(); err != nil {
		return false, err
	}
	current, err := readReportRecoveryRaw(ctx, tx, claim.ReportID, true)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(current, observed) {
		return false, ErrReportPrivacy
	}
	currentJournal, err := readReportRecoveryJournal(ctx, tx, claim.ReportID, true)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(currentJournal, journal) {
		return false, ErrReportPrivacy
	}
	for _, record := range currentJournal {
		if !validReportRecoveryRecord(record) {
			return false, ErrReportPrivacy
		}
		if record.Disposition == "REDACTED" {
			return false, ErrReportPrivacy
		}
		if record.SubjectID != nil && *record.SubjectID != claim.ReporterSubject.ID && (claim.TargetSubject == nil || *record.SubjectID != claim.TargetSubject.ID) {
			var ended bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, *record.SubjectID).Scan(&ended); err != nil {
				return false, err
			}
			if ended {
				return false, ErrReportPrivacy
			} // Owning terminal repair precedes ordinary rejection.
		}
	}
	if fact.TargetType != claim.TargetType || fact.TargetID != claim.TargetID {
		return false, errors.New("target claim mismatch")
	}
	if fact.ReporterProvenance != "VERIFIED_AT_SUBMISSION" || fact.ReporterAccountID == nil || fact.ReporterSubjectID == nil || fact.ReporterSubjectRevision == nil || *fact.ReporterAccountID != *claim.ReporterAccountID || *fact.ReporterSubjectID != *claim.ReporterSubjectID || *fact.ReporterSubjectRevision != *claim.ReporterSubjectRevision {
		return false, ErrReportPrivacy
	}
	if terminal {
		// The target account column may still be NULL: the verified late result is
		// sufficient to scrub this pending row, never to recreate a terminal link.
		for _, binding := range terminalBindings {
			if binding.ID == claim.ReporterSubject.ID {
				if err = s.recordTerminalReportSlotTx(ctx, tx, fact.ReportID, "REPORTER", binding.ID); err != nil {
					return false, err
				}
			}
			if claim.TargetSubject != nil && binding.ID == claim.TargetSubject.ID {
				if err = s.recordTerminalReportSlotTx(ctx, tx, fact.ReportID, "USER_TARGET", binding.ID); err != nil {
					return false, err
				}
			}
		}
		if err = finishReportPurgeTx(ctx, tx, []uuid.UUID{fact.ReportID}); err != nil {
			return false, err
		}
		if err = claim.PrivacyCheck(ctx); err != nil {
			return false, err
		}
		return false, tx.Commit(ctx)
	}
	// Soft deletion after acceptance does not retrospectively change reporter
	// eligibility. Its authoritative purge deadline still closes activation.
	if claim.TargetSubject != nil {
		if claim.TargetSubject.State != "LIVE" {
			if _, err = tx.Exec(ctx, `UPDATE reports SET validation_claim=NULL,validation_claim_until=NULL,validation_next_at=now()+interval '1 second',validation_failure_code='SUBJECT_NOT_LIVE' WHERE id=$1`, fact.ReportID); err != nil {
				return false, err
			}
			if err = claim.PrivacyCheck(ctx); err != nil {
				return false, err
			}
			if err = checkRetained(); err != nil {
				return false, err
			}
			return false, tx.Commit(ctx)
		}
		if err = s.Subjects.RequireLiveSubjectTx(ctx, tx, claim.TargetSubject.AccountID, claim.TargetSubject.ID, claim.TargetSubject.Revision); err != nil {
			return false, err
		}
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
		if err = requireNoReportTargetRejectionTx(ctx, tx, fact.ReportID); err != nil {
			return false, err
		}
		if fact.TargetType == "user" {
			err = recordReportTargetRejectionTx(ctx, tx, ReportTargetRejection{ReportID: fact.ReportID, Decision: state, Provenance: "OWNER_VALIDATION", SchemaVersion: 1})
			if err == nil {
				err = minimizeReportUserTargetTx(ctx, tx, fact.ReportID, state)
			}
		} else {
			_, err = tx.Exec(ctx, `UPDATE reports SET validation_state=$2,status='dismissed',description=NULL,evidence='{}'::jsonb,
 resolution=jsonb_build_object('code',$3::text),resolved_at=now(),updated_at=now(),validation_claim=NULL,
 validation_claim_until=NULL,validation_failure_code=$3 WHERE id=$1`, fact.ReportID, state, "TARGET_"+map[string]string{"MISSING": "NOT_FOUND", "DELETED": "DELETED"}[state])
		}
	} else {
		if accountID != uuid.Nil {
			fact.TargetAccountID = &accountID
			fact.TargetSubjectID = &claim.TargetSubject.ID
			fact.TargetSubjectRevision = &claim.TargetSubject.Revision
			fact.TargetProvenance = "VERIFIED_AT_VALIDATION"
		}
		_, err = tx.Exec(ctx, `UPDATE reports SET validation_state='VALID',validated_target_account_id=$2,
 validated_target_subject_id=$3,validated_target_subject_revision=$4,validated_target_provenance=$5,validation_claim=NULL,validation_claim_until=NULL,validation_failure_code=NULL,updated_at=now() WHERE id=$1`, fact.ReportID, fact.TargetAccountID, fact.TargetSubjectID, fact.TargetSubjectRevision, map[bool]string{true: "VERIFIED_AT_VALIDATION", false: "LEGACY_UNVERIFIED"}[accountID != uuid.Nil])
		if err == nil && accountID != uuid.Nil {
			err = s.RecordReportRecoveryTx(ctx, tx, ReportRecoveryRecord{ReportID: fact.ReportID, Slot: "USER_TARGET", Disposition: "BOUND", SubjectID: fact.TargetSubjectID, EvidenceKind: "AUTH_SUBJECT", EvidenceID: *fact.TargetSubjectID, EvidenceVersion: 1})
		}
		// Finish every subject-journal write before locking the rejection family.
		// A restored pending intake cannot reactivate against a current receipt.
		if err == nil {
			err = requireNoReportTargetRejectionTx(ctx, tx, fact.ReportID)
		}
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
	if err = claim.PrivacyCheck(ctx); err != nil {
		return false, err
	}
	// Hooks and event serialization may cross a retained deadline. Recheck the
	// authoritative database clock and exact bindings immediately before commit.
	if err = checkRetained(); err != nil {
		return false, err
	}
	if claim.TargetSubject != nil {
		if err = s.Subjects.RequireLiveSubjectTx(ctx, tx, claim.TargetSubject.AccountID, claim.TargetSubject.ID, claim.TargetSubject.Revision); err != nil {
			return false, errors.Join(ErrReportPrivacy, err)
		}
	}
	if err = checkLease(); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *ReportStore) readReportValidationObservation(ctx context.Context, id uuid.UUID) (reportRecoveryRaw, []ReportRecoveryRecord, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return reportRecoveryRaw{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	raw, err := readReportRecoveryRaw(ctx, tx, id, false)
	if err != nil {
		return raw, nil, err
	}
	journal, err := readReportRecoveryJournal(ctx, tx, id, false)
	if err != nil {
		return raw, nil, err
	}
	return raw, journal, tx.Commit(ctx)
}
