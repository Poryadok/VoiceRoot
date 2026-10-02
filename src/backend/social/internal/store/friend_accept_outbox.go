package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DispatchAcceptedFriendOutbox publishes committed friendship transitions.
// Each row is locked until its publish succeeds and the delivery mark commits.
// A crash after publish but before commit may replay the event; Chat applies it
// idempotently after checking the current friendship.
func (s *FriendshipStore) DispatchAcceptedFriendOutbox(ctx context.Context, limit int, publish func(context.Context, uuid.UUID, uuid.UUID) error) (int, error) {
	if s == nil || s.Pool == nil {
		return 0, errors.New("friendship store unavailable")
	}
	if publish == nil {
		return 0, errors.New("friend acceptance publisher unavailable")
	}
	if limit <= 0 {
		return 0, nil
	}
	var delivered int
	for delivered < limit {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return delivered, err
		}
		var friendshipID, requester, target uuid.UUID
		err = tx.QueryRow(ctx, `
SELECT friendship_id, requester_profile_id, target_profile_id
FROM friend_accept_outbox
WHERE delivered_at IS NULL
ORDER BY created_at, friendship_id
LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&friendshipID, &requester, &target)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			return delivered, nil
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return delivered, err
		}
		if err := publish(ctx, requester, target); err != nil {
			_ = tx.Rollback(ctx)
			return delivered, err
		}
		_, err = tx.Exec(ctx, `UPDATE friend_accept_outbox SET delivered_at = now() WHERE friendship_id = $1`, friendshipID)
		if err != nil {
			_ = tx.Rollback(ctx)
			return delivered, err
		}
		if err := tx.Commit(ctx); err != nil {
			return delivered, err
		}
		delivered++
	}
	return delivered, nil
}
