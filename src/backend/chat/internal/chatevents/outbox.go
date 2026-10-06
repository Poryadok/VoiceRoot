package chatevents

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	chatDeletedOutboxPollInterval = time.Second
	chatDeletedOutboxLease        = 30 * time.Second
	chatDeletedOutboxBatchSize    = 32
	chatDeletedOutboxRunLimit     = 8
	chatDeletedOutboxMaxBackoff   = time.Minute
)

type chatDeletedBytesPublisher interface {
	PublishChatDeletedBytes(context.Context, string, []byte) error
}

// ChatDeletedOutbox retries immutable purge events until JetStream confirms them.
// It owns no event construction and never changes the bytes committed by PurgeSpace.
type ChatDeletedOutbox struct {
	Pool      *pgxpool.Pool
	Publisher chatDeletedBytesPublisher
}

type chatDeletedOutboxEntry struct {
	ID       uuid.UUID
	Token    uuid.UUID
	Bytes    []byte
	Attempts int64
}

func (w *ChatDeletedOutbox) Run(ctx context.Context) error {
	if w == nil || w.Pool == nil || w.Publisher == nil {
		return errors.New("chat deleted outbox is not configured")
	}
	ticker := time.NewTicker(chatDeletedOutboxPollInterval)
	defer ticker.Stop()
	for {
		if err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			// A failed batch is retried on the next ordinary poll; durable rows remain pending.
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// RunOnce performs one bounded delivery pass and one bounded retention cleanup.
func (w *ChatDeletedOutbox) RunOnce(ctx context.Context) error {
	if w == nil || w.Pool == nil || w.Publisher == nil {
		return errors.New("chat deleted outbox is not configured")
	}
	if err := w.runBatch(ctx); err != nil {
		return err
	}
	return w.cleanupPublished(ctx)
}

func (w *ChatDeletedOutbox) runBatch(ctx context.Context) error {
	for i := 0; i < chatDeletedOutboxRunLimit; i++ {
		entry, err := w.claim(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		publishErr := w.Publisher.PublishChatDeletedBytes(ctx, entry.ID.String(), entry.Bytes)
		if publishErr == nil {
			if err := w.ack(ctx, entry); err != nil {
				return err
			}
			continue
		}
		if err := w.fail(ctx, entry); err != nil {
			return err
		}
		// Avoid retrying a just-failed row in this same run; its persisted backoff
		// governs the next attempt.
		return nil
	}
	return nil
}

func (w *ChatDeletedOutbox) claim(ctx context.Context) (chatDeletedOutboxEntry, error) {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return chatDeletedOutboxEntry{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	entry := chatDeletedOutboxEntry{Token: uuid.New()}
	err = tx.QueryRow(ctx, `WITH candidate AS (
		SELECT event_id FROM chat_deleted_event_outbox
		WHERE published_at IS NULL AND next_attempt_at <= clock_timestamp()
		  AND (lease_until IS NULL OR lease_until <= clock_timestamp())
		ORDER BY next_attempt_at,created_at,event_id
		FOR UPDATE SKIP LOCKED LIMIT 1
	)
	UPDATE chat_deleted_event_outbox AS outbox SET lease_token=$1,
		lease_until=clock_timestamp()+$2::interval,
		attempt_count=CASE WHEN outbox.attempt_count < 9223372036854775807 THEN outbox.attempt_count+1 ELSE outbox.attempt_count END
	FROM candidate WHERE outbox.event_id=candidate.event_id
	RETURNING outbox.event_id,outbox.event_bytes,outbox.attempt_count`, entry.Token, chatDeletedOutboxLease.String()).Scan(&entry.ID, &entry.Bytes, &entry.Attempts)
	if err != nil {
		return chatDeletedOutboxEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return chatDeletedOutboxEntry{}, err
	}
	return entry, nil
}

func (w *ChatDeletedOutbox) ack(ctx context.Context, entry chatDeletedOutboxEntry) error {
	_, err := w.Pool.Exec(ctx, `UPDATE chat_deleted_event_outbox
		SET published_at=clock_timestamp(),lease_token=NULL,lease_until=NULL,last_error_class=NULL
		WHERE event_id=$1 AND published_at IS NULL AND lease_token=$2`, entry.ID, entry.Token)
	return err
}

func (w *ChatDeletedOutbox) fail(ctx context.Context, entry chatDeletedOutboxEntry) error {
	backoff := chatDeletedOutboxBackoff(entry.Attempts)
	_, err := w.Pool.Exec(ctx, `UPDATE chat_deleted_event_outbox SET
		lease_token=NULL,lease_until=NULL,last_error_class='publish_failed',
		next_attempt_at=clock_timestamp()+$3::interval
		WHERE event_id=$1 AND published_at IS NULL AND lease_token=$2`, entry.ID, entry.Token, backoff.String())
	return err
}

func chatDeletedOutboxBackoff(attempt int64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := time.Second << min(attempt-1, int64(6))
	if backoff > chatDeletedOutboxMaxBackoff {
		return chatDeletedOutboxMaxBackoff
	}
	return backoff
}

func (w *ChatDeletedOutbox) cleanupPublished(ctx context.Context) error {
	_, err := w.Pool.Exec(ctx, `WITH expired AS (
		SELECT outbox.event_id FROM chat_deleted_event_outbox outbox
		JOIN chat_space_lifecycle_operations parent
		  ON parent.space_id=outbox.space_id
		 AND parent.deletion_operation_id=outbox.deletion_operation_id
		 AND parent.generation=outbox.generation
		 AND parent.operation_kind='PURGE'
		WHERE outbox.published_at IS NOT NULL
		  AND parent.retain_until <= clock_timestamp()
		ORDER BY outbox.event_id
		FOR UPDATE OF outbox SKIP LOCKED LIMIT $1
	)
	DELETE FROM chat_deleted_event_outbox outbox USING expired
	WHERE outbox.event_id=expired.event_id AND outbox.published_at IS NOT NULL
	  AND outbox.lease_token IS NULL`, chatDeletedOutboxBatchSize)
	return err
}
