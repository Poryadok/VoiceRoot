package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DeliverIntakeEvent serializes deletion with the bounded publication attempt.
// A purge committed before this lock cannot leave an independently publishable
// payload. An uncertain ACK retries the identical durable ID and bytes.
var errIntakeDispatchMoved = errors.New("intake dispatch candidate changed")

func (s *ReportStore) DeliverIntakeEvent(ctx context.Context, publish func(context.Context, IntakeEvent) error) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errStoreNotConfigured
	}
	check, err := AcquireReportPrivacy(ctx, s.Privacy)
	if err != nil {
		return false, err
	}
	for {
		delivered, err := s.deliverIntakeEvent(ctx, publish, check)
		if !errors.Is(err, errIntakeDispatchMoved) {
			return delivered, err
		}
		if err = ctx.Err(); err != nil {
			return false, err
		}
		// Another dispatcher or purge won the optimistic selection. Rollback has
		// released every lock; select again under the SAME captured generation.
	}
}

func (s *ReportStore) deliverIntakeEvent(ctx context.Context, publish func(context.Context, IntakeEvent) error, check ReportPrivacyCheck) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errStoreNotConfigured
	}
	if publish == nil {
		return false, errors.New("intake publisher unavailable")
	}
	if err := check(ctx); err != nil {
		return false, err
	}
	// Read candidates without a row lock. Account and subject locks must precede
	// report locks, including on dispatch, so purge cannot deadlock publication.
	var candidate uuid.UUID
	var reporterAccount, reporterSubject, targetAccount, targetSubject *uuid.UUID
	var reporterRevision, targetRevision *int64
	err := s.Pool.QueryRow(ctx, `SELECT r.id,r.reporter_account_id,r.reporter_subject_id,r.reporter_subject_revision,r.validated_target_account_id,r.validated_target_subject_id,r.validated_target_subject_revision
 FROM reports r WHERE r.validation_state='VALID' AND EXISTS(SELECT 1 FROM report_intake_outbox o WHERE o.report_id=r.id AND o.delivered_at IS NULL AND o.available_at<=now()) ORDER BY r.created_at,r.id LIMIT 1`).Scan(&candidate, &reporterAccount, &reporterSubject, &reporterRevision, &targetAccount, &targetSubject, &targetRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if s.Subjects == nil || reporterAccount == nil || reporterSubject == nil || reporterRevision == nil {
		return false, ErrReportPrivacy
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var event IntakeEvent
	var reportID uuid.UUID
	var attempts int
	accounts := []uuid.UUID{*reporterAccount}
	if targetAccount != nil {
		accounts = append(accounts, *targetAccount)
	}
	if err = lockReportAccounts(ctx, tx, accounts...); err != nil {
		return false, err
	}
	bindings := []reportDispatchBinding{{*reporterAccount, *reporterSubject, *reporterRevision}}
	if targetAccount != nil {
		if targetSubject == nil || targetRevision == nil {
			return false, ErrReportPrivacy
		}
		bindings = append(bindings, reportDispatchBinding{*targetAccount, *targetSubject, *targetRevision})
	}
	if err = lockDispatchSubjects(ctx, tx, bindings); err != nil {
		return false, err
	}
	err = tx.QueryRow(ctx, `SELECT r.id FROM reports r WHERE r.id=$1 AND r.validation_state='VALID'
 AND r.reporter_account_id=$2 AND r.reporter_subject_id=$3 AND r.reporter_subject_revision=$4
 AND r.validated_target_account_id IS NOT DISTINCT FROM $5::uuid AND r.validated_target_subject_id IS NOT DISTINCT FROM $6::uuid AND r.validated_target_subject_revision IS NOT DISTINCT FROM $7::bigint
 AND EXISTS(SELECT 1 FROM report_intake_outbox o WHERE o.report_id=r.id AND o.delivered_at IS NULL AND o.available_at<=now())
 FOR UPDATE OF r SKIP LOCKED`, candidate, reporterAccount, reporterSubject, reporterRevision, targetAccount, targetSubject, targetRevision).Scan(&reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, errIntakeDispatchMoved
	}
	if err != nil {
		return false, err
	}
	var privacyDeadline *time.Time
	for _, binding := range bindings {
		var live bool
		var purgeAt *time.Time
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_subjects a WHERE a.account_id=$1 AND a.subject_id=$2 AND a.lifecycle_revision >= $3
 AND ((a.lifecycle_state='LIVE' AND a.purge_at IS NULL AND a.deletion_cycle_id IS NULL)
 OR (a.lifecycle_state='DELETED' AND a.purge_at>clock_timestamp() AND a.deletion_cycle_id IS NOT NULL
 AND a.deletion_cycle_id<>'00000000-0000-0000-0000-000000000000'::uuid))
 AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=a.subject_id)),
 (SELECT purge_at FROM automod_subjects WHERE account_id=$1 AND subject_id=$2)`, binding.account, binding.subject, binding.revision).Scan(&live, &purgeAt); err != nil {
			return false, err
		}
		if !live {
			return false, ErrReportPrivacy
		}
		if purgeAt != nil && (privacyDeadline == nil || purgeAt.Before(*privacyDeadline)) {
			privacyDeadline = purgeAt
		}
	}
	err = tx.QueryRow(ctx, `SELECT id,kind,subject,payload,attempts FROM report_intake_outbox WHERE report_id=$1
 AND delivered_at IS NULL AND available_at<=now() ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, reportID).Scan(&event.ID, &event.Kind, &event.Subject, &event.Payload, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, errIntakeDispatchMoved
	}
	if err != nil {
		return false, err
	}
	attemptDeadline := time.Now().Add(5 * time.Second)
	if privacyDeadline != nil && privacyDeadline.Before(attemptDeadline) {
		attemptDeadline = *privacyDeadline
	}
	attemptCtx, cancel := context.WithDeadline(ctx, attemptDeadline)
	if err = check(attemptCtx); err != nil {
		cancel()
		return false, err
	}
	if err = attemptCtx.Err(); err != nil {
		cancel()
		return false, err
	}
	publishErr := publish(attemptCtx, event)
	if publishErr == nil {
		publishErr = attemptCtx.Err()
	}
	cancel()
	if publishErr == nil {
		_, err = tx.Exec(ctx, `UPDATE report_intake_outbox SET delivered_at=now(),failure_code=NULL WHERE id=$1`, event.ID)
	} else {
		delay := time.Second
		for i := 0; i < attempts && delay < 5*time.Minute; i++ {
			delay *= 2
		}
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		_, err = tx.Exec(ctx, `UPDATE report_intake_outbox SET attempts=CASE WHEN attempts<2147483647 THEN attempts+1 ELSE attempts END,
 available_at=now()+interval '1 second'*$2,failure_code='PUBLISH_UNAVAILABLE' WHERE id=$1`, event.ID, delay.Seconds())
	}
	if err != nil {
		return false, err
	}
	if err = check(ctx); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, publishErr
}
