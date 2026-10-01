package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DispatchFriendRequestOutbox publishes pending friend-request notifications.
// The request and event IDs are stable across retries; a new explicit invitation
// gets a new event ID while reusing the friendship row's request ID.
func (s *FriendshipStore) DispatchFriendRequestOutbox(
	ctx context.Context,
	limit int,
	publish func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error,
) (int, error) {
	if s == nil || s.Pool == nil {
		return 0, errors.New("friendship store unavailable")
	}
	if publish == nil {
		return 0, errors.New("friend request publisher unavailable")
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
		var friendshipID, eventID, requester, target uuid.UUID
		err = tx.QueryRow(ctx, `
SELECT friendship_id, event_id, requester_profile_id, target_profile_id
FROM friend_request_outbox
WHERE delivered_at IS NULL AND cancelled_at IS NULL
ORDER BY created_at, friendship_id
LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&friendshipID, &eventID, &requester, &target)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			return delivered, nil
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return delivered, err
		}
		if err := publish(ctx, friendshipID, eventID, requester, target); err != nil {
			_ = tx.Rollback(ctx)
			return delivered, err
		}
		_, err = tx.Exec(ctx, `
UPDATE friend_request_outbox SET delivered_at = now()
WHERE friendship_id = $1 AND event_id = $2`, friendshipID, eventID)
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
