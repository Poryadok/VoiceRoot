package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BlockStore persists account-level blocks in social_db.blocks.
type BlockStore struct {
	Pool *pgxpool.Pool
}

// BlockAccount inserts a block row if absent. Idempotent when the pair already exists.
func (s *BlockStore) BlockAccount(ctx context.Context, blockerAccountID, blockedAccountID uuid.UUID) error {
	return s.BlockAccountAndSeverFriendships(ctx, blockerAccountID, blockedAccountID, nil, nil)
}

// BlockAccountAndSeverFriendships inserts the block row and, when profile sets are provided,
// deletes all friendship rows (accepted, pending, declined) between any profile in setA and setB.
// The block insert and friendship cleanup run in one transaction.
func (s *BlockStore) BlockAccountAndSeverFriendships(ctx context.Context, blockerAccountID, blockedAccountID uuid.UUID, blockerProfiles, blockedProfiles []uuid.UUID) error {
	if blockerAccountID == blockedAccountID {
		return ErrSelfBlock
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
INSERT INTO blocks (blocker_account_id, blocked_account_id)
VALUES ($1, $2)
ON CONFLICT (blocker_account_id, blocked_account_id) DO NOTHING`,
		blockerAccountID, blockedAccountID)
	if err != nil {
		return err
	}

	if len(blockerProfiles) > 0 && len(blockedProfiles) > 0 {
		_, err = tx.Exec(ctx, `
DELETE FROM friendships
WHERE (
  requester_profile_id = ANY($1::uuid[]) AND target_profile_id = ANY($2::uuid[])
) OR (
  requester_profile_id = ANY($2::uuid[]) AND target_profile_id = ANY($1::uuid[])
)`, blockerProfiles, blockedProfiles)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// UnblockAccount deletes the block row for the ordered pair.
func (s *BlockStore) UnblockAccount(ctx context.Context, blockerAccountID, blockedAccountID uuid.UUID) error {
	cmd, err := s.Pool.Exec(ctx, `
DELETE FROM blocks
WHERE blocker_account_id = $1 AND blocked_account_id = $2`,
		blockerAccountID, blockedAccountID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrBlockNotFound
	}
	return nil
}

// BlockedRow is one blocked account from the blocker's perspective.
type BlockedRow struct {
	BlockID          uuid.UUID
	BlockedAccountID uuid.UUID
	CreatedAt        time.Time
}

// BlocksListCursor continues ListBlocked after (CreatedAt, ID) in descending order.
type BlocksListCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// ListBlocked returns rows for blockerAccountID ordered by created_at DESC, id DESC.
func (s *BlockStore) ListBlocked(ctx context.Context, blockerAccountID uuid.UUID, after *BlocksListCursor, limit int) ([]BlockedRow, error) {
	if limit <= 0 {
		limit = 20
	}
	var (
		q    string
		args []any
	)
	if after == nil {
		q = `
SELECT id, blocked_account_id, created_at
FROM blocks
WHERE blocker_account_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`
		args = []any{blockerAccountID, limit + 1}
	} else {
		q = `
SELECT id, blocked_account_id, created_at
FROM blocks
WHERE blocker_account_id = $1
  AND (created_at, id) < ($2::timestamptz, $3::uuid)
ORDER BY created_at DESC, id DESC
LIMIT $4`
		args = []any{blockerAccountID, after.CreatedAt, after.ID, limit + 1}
	}
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BlockedRow
	for rows.Next() {
		var r BlockedRow
		if err := rows.Scan(&r.BlockID, &r.BlockedAccountID, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// DirectedBlockExists reports whether blockerAccountID has blocked blockedAccountID (ordered pair).
func (s *BlockStore) DirectedBlockExists(ctx context.Context, blockerAccountID, blockedAccountID uuid.UUID) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM blocks
  WHERE blocker_account_id = $1 AND blocked_account_id = $2
)`, blockerAccountID, blockedAccountID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
