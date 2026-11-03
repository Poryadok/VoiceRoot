package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DispatchBlockEventOutbox retries committed revocations independently of RPCs.
// Row locks serialize workers. A failed/uncertain publish or failed DB commit
// leaves the same immutable event pending, including after process restart.
func (s *BlockStore) DispatchBlockEventOutbox(ctx context.Context, limit int, publish func(context.Context, uuid.UUID, []byte) error) (int, error) {
	if s == nil || s.Pool == nil || publish == nil {
		return 0, errors.New("block outbox dependencies unavailable")
	}
	for delivered := 0; delivered < limit; delivered++ {
		found, err := s.dispatchBlockEvent(ctx, publish)
		if err != nil || !found {
			return delivered, err
		}
	}
	return max(0, limit), nil
}

func (s *BlockStore) dispatchBlockEvent(ctx context.Context, publish func(context.Context, uuid.UUID, []byte) error) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var id uuid.UUID
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT event_id, payload FROM block_event_outbox
WHERE delivered_at IS NULL ORDER BY created_at, event_id
LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	send, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = publish(send, id, payload)
	cancel()
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE block_event_outbox SET delivered_at = now() WHERE event_id = $1`, id); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
