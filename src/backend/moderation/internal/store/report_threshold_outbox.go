package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DeliverEvent uses the shared account lock through one bounded ACK attempt.
// Terminal purge and closure serialize before it; unknown/restored subjects do
// not publish until the shared lifecycle reconciliation has made them live.
func (s *ReportThresholdStore) DeliverEvent(ctx context.Context, publish func(context.Context, ReportThresholdEvent) error) (bool, error) {
	if s == nil || s.Pool == nil || s.Sanctions == nil || publish == nil {
		return false, errStoreNotConfigured
	}
	var account uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT c.account_id FROM report_threshold_cases c JOIN automod_subjects a ON a.subject_id=c.subject_id AND a.account_id=c.account_id
WHERE c.closed_at IS NULL AND a.lifecycle_state='LIVE' AND a.purge_at IS NULL
AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=c.subject_id)
AND EXISTS(SELECT 1 FROM report_threshold_outbox o WHERE o.case_id=c.id AND o.delivered_at IS NULL AND o.cancelled_at IS NULL AND o.available_at<=clock_timestamp())
ORDER BY c.created_at,c.id LIMIT 1`).Scan(&account)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	worked := false
	var publishErr error
	err = s.Sanctions.WithAccountLock(ctx, account, func(tx pgx.Tx) error {
		var event ReportThresholdEvent
		var attempts int
		err := tx.QueryRow(ctx, `SELECT o.id,o.kind,o.subject,o.payload,o.attempts FROM report_threshold_outbox o
JOIN report_threshold_cases c ON c.id=o.case_id JOIN automod_subjects a ON a.subject_id=c.subject_id AND a.account_id=c.account_id
WHERE c.account_id=$1 AND c.closed_at IS NULL AND a.lifecycle_state='LIVE' AND a.purge_at IS NULL
AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=c.subject_id)
AND EXISTS(SELECT 1 FROM sanctions s WHERE s.machine_case_id=c.id AND s.type='shadow_ban' AND s.revoked_at IS NULL AND (s.expires_at IS NULL OR s.expires_at>clock_timestamp()))
AND o.delivered_at IS NULL AND o.cancelled_at IS NULL AND o.available_at<=clock_timestamp()
ORDER BY o.created_at,o.id FOR UPDATE OF o SKIP LOCKED LIMIT 1`, account).Scan(&event.ID, &event.Kind, &event.Subject, &event.Payload, &attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		worked = true
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		publishErr = publish(attemptCtx, event)
		cancel()
		if publishErr == nil {
			_, err = tx.Exec(ctx, `UPDATE report_threshold_outbox SET delivered_at=clock_timestamp(),failure_code=NULL WHERE id=$1`, event.ID)
		} else {
			delay := time.Second
			for i := 0; i < attempts && delay < 5*time.Minute; i++ {
				delay *= 2
			}
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
			_, err = tx.Exec(ctx, `UPDATE report_threshold_outbox SET attempts=LEAST(attempts::bigint+1,2147483647)::integer,
available_at=clock_timestamp()+interval '1 second'*$2,failure_code='PUBLISH_UNAVAILABLE' WHERE id=$1`, event.ID, delay.Seconds())
		}
		return err
	})
	return worked, errors.Join(err, publishErr)
}
