package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BlockStore persists account-level blocks in social_db.blocks.
type BlockStore struct {
	Pool *pgxpool.Pool
}

// BlockAccount inserts a block row if absent. Idempotent when the pair already exists.
func (s *BlockStore) BlockAccount(ctx context.Context, blockerAccountID, blockedAccountID uuid.UUID) error {
	if blockerAccountID == blockedAccountID {
		return ErrSelfBlock
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO blocks (blocker_account_id, blocked_account_id)
VALUES ($1, $2)
ON CONFLICT (blocker_account_id, blocked_account_id) DO NOTHING`,
		blockerAccountID, blockedAccountID)
	return err
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
