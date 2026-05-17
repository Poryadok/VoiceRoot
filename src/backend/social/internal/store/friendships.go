package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FriendshipStore persists friendships in social_db (see migrations/social_db).
type FriendshipStore struct {
	Pool *pgxpool.Pool
}

// SendInvitation creates or refreshes a pending invitation from requester to target.
// Idempotent when the same ordered pair is already pending.
// Re-opens a declined row in the same direction (friends.md: declined visible to sender; new invite allowed).
func (s *FriendshipStore) SendInvitation(ctx context.Context, requester, target uuid.UUID) error {
	if requester == target {
		return ErrSelfInvitation
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var acceptedID uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT id FROM friendships
WHERE status = 'accepted'
  AND (
    (requester_profile_id = $1 AND target_profile_id = $2)
    OR (requester_profile_id = $2 AND target_profile_id = $1)
  )
LIMIT 1`, requester, target).Scan(&acceptedID)
	if err == nil {
		return ErrAlreadyFriends
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var revID uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT id FROM friendships
WHERE status = 'pending'
  AND requester_profile_id = $2 AND target_profile_id = $1
LIMIT 1`, requester, target).Scan(&revID)
	if err == nil {
		return ErrIncomingPendingExists
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var rowID uuid.UUID
	var st string
	err = tx.QueryRow(ctx, `
SELECT id, status FROM friendships
WHERE requester_profile_id = $1 AND target_profile_id = $2
LIMIT 1`, requester, target).Scan(&rowID, &st)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		switch st {
		case "pending":
			return tx.Commit(ctx)
		case "accepted":
			return ErrAlreadyFriends
		case "declined":
			_, err = tx.Exec(ctx, `
UPDATE friendships SET status = 'pending', updated_at = now() WHERE id = $1`, rowID)
			if err != nil {
				return err
			}
			return tx.Commit(ctx)
		default:
			return errors.New("unexpected friendship status")
		}
	}

	_, err = tx.Exec(ctx, `
INSERT INTO friendships (requester_profile_id, target_profile_id, status)
VALUES ($1, $2, 'pending')`, requester, target)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AcceptInvitation marks the pending row (requester -> caller) as accepted.
func (s *FriendshipStore) AcceptInvitation(ctx context.Context, caller, requester uuid.UUID) error {
	cmd, err := s.Pool.Exec(ctx, `
UPDATE friendships
SET status = 'accepted', updated_at = now()
WHERE status = 'pending'
  AND requester_profile_id = $1 AND target_profile_id = $2`, requester, caller)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrFriendshipNotFound
	}
	return nil
}

// DeclineInvitation marks the pending row (requester -> caller) as declined.
func (s *FriendshipStore) DeclineInvitation(ctx context.Context, caller, requester uuid.UUID) error {
	cmd, err := s.Pool.Exec(ctx, `
UPDATE friendships
SET status = 'declined', updated_at = now()
WHERE status = 'pending'
  AND requester_profile_id = $1 AND target_profile_id = $2`, requester, caller)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrFriendshipNotFound
	}
	return nil
}

// PendingFriendIncoming is an incoming pending request (someone invited caller).
type PendingFriendIncoming struct {
	RequesterProfileID uuid.UUID
	CreatedAt          time.Time
}

// PendingFriendOutgoing is an outgoing request from caller (pending or declined, visible to sender).
type PendingFriendOutgoing struct {
	TargetProfileID uuid.UUID
	CreatedAt       time.Time
	Status          string
}

// ListFriendRequests returns incoming pending and outgoing pending+declined for profileID.
func (s *FriendshipStore) ListFriendRequests(ctx context.Context, profileID uuid.UUID) (incoming []PendingFriendIncoming, outgoing []PendingFriendOutgoing, err error) {
	rows, err := s.Pool.Query(ctx, `
SELECT requester_profile_id, created_at
FROM friendships
WHERE status = 'pending' AND target_profile_id = $1
ORDER BY created_at DESC`, profileID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r PendingFriendIncoming
		if err := rows.Scan(&r.RequesterProfileID, &r.CreatedAt); err != nil {
			return nil, nil, err
		}
		incoming = append(incoming, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	orows, err := s.Pool.Query(ctx, `
SELECT target_profile_id, created_at, status
FROM friendships
WHERE requester_profile_id = $1 AND status IN ('pending', 'declined')
ORDER BY created_at DESC`, profileID)
	if err != nil {
		return nil, nil, err
	}
	defer orows.Close()
	for orows.Next() {
		var o PendingFriendOutgoing
		if err := orows.Scan(&o.TargetProfileID, &o.CreatedAt, &o.Status); err != nil {
			return nil, nil, err
		}
		outgoing = append(outgoing, o)
	}
	if err := orows.Err(); err != nil {
		return nil, nil, err
	}
	return incoming, outgoing, nil
}

// FriendEdgeRow is one accepted friendship edge from caller's perspective.
type FriendEdgeRow struct {
	FriendshipID   uuid.UUID
	OtherProfileID uuid.UUID
	FriendsSince   time.Time
}

// FriendsListCursor continues ListFriends after (UpdatedAt, ID) descending order.
type FriendsListCursor struct {
	UpdatedAt time.Time
	ID        uuid.UUID
}

// ListFriends returns accepted friendships where profileID participates, ordered by updated_at DESC, id DESC.
func (s *FriendshipStore) ListFriends(ctx context.Context, profileID uuid.UUID, after *FriendsListCursor, limit int) ([]FriendEdgeRow, error) {
	if limit <= 0 {
		limit = 20
	}
	args := []any{profileID, profileID, limit + 1}
	var q string
	if after == nil {
		q = `
SELECT id,
       CASE WHEN requester_profile_id = $1 THEN target_profile_id ELSE requester_profile_id END AS other,
       updated_at
FROM friendships
WHERE status = 'accepted' AND (requester_profile_id = $1 OR target_profile_id = $2)
ORDER BY updated_at DESC, id DESC
LIMIT $3`
	} else {
		q = `
SELECT id,
       CASE WHEN requester_profile_id = $1 THEN target_profile_id ELSE requester_profile_id END AS other,
       updated_at
FROM friendships
WHERE status = 'accepted' AND (requester_profile_id = $1 OR target_profile_id = $2)
  AND (updated_at, id) < ($4::timestamptz, $5::uuid)
ORDER BY updated_at DESC, id DESC
LIMIT $3`
		args = []any{profileID, profileID, limit + 1, after.UpdatedAt, after.ID}
	}
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FriendEdgeRow
	for rows.Next() {
		var row FriendEdgeRow
		if err := rows.Scan(&row.FriendshipID, &row.OtherProfileID, &row.FriendsSince); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// RemoveFriend deletes an accepted friendship row between caller and friend (unordered).
func (s *FriendshipStore) RemoveFriend(ctx context.Context, caller, friend uuid.UUID) error {
	cmd, err := s.Pool.Exec(ctx, `
DELETE FROM friendships
WHERE status = 'accepted'
  AND (
    (requester_profile_id = $1 AND target_profile_id = $2)
    OR (requester_profile_id = $2 AND target_profile_id = $1)
  )`, caller, friend)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrFriendshipNotFound
	}
	return nil
}

// AreFriendsAccepted reports whether profileA and profileB have an accepted friendship (unordered pair).
func (s *FriendshipStore) AreFriendsAccepted(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if profileA == profileB {
		return false, nil
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM friendships
  WHERE status = 'accepted'
    AND (
      (requester_profile_id = $1 AND target_profile_id = $2)
      OR (requester_profile_id = $2 AND target_profile_id = $1)
    )
)`, profileA, profileB).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
