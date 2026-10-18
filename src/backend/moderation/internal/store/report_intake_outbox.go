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
func (s *ReportStore) DeliverIntakeEvent(ctx context.Context, publish func(context.Context, IntakeEvent) error) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errStoreNotConfigured
	}
	if publish == nil {
		return false, errors.New("intake publisher unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var event IntakeEvent
	var reportID uuid.UUID
	var attempts int
	// Lock the parent first, consistently with deletion, then the event. SKIP
	// LOCKED prevents a slow attempt from blocking unrelated intake work.
	err = tx.QueryRow(ctx, `SELECT r.id FROM reports r WHERE r.validation_state='VALID'
 AND EXISTS(SELECT 1 FROM report_intake_outbox o WHERE o.report_id=r.id AND o.delivered_at IS NULL AND o.available_at<=now())
 ORDER BY r.created_at,r.id FOR UPDATE OF r SKIP LOCKED LIMIT 1`).Scan(&reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = tx.QueryRow(ctx, `SELECT id,kind,subject,payload,attempts FROM report_intake_outbox WHERE report_id=$1
 AND delivered_at IS NULL AND available_at<=now() ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, reportID).Scan(&event.ID, &event.Kind, &event.Subject, &event.Payload, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	publishErr := publish(attemptCtx, event)
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
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, publishErr
}
