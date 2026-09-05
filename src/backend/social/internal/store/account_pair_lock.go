package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// lockAccountPair serializes account-scoped blocks with friendship writes for
// the same unordered account pair. The transaction-scoped advisory lock keeps
// the block cascade and the final block recheck in one linearization domain.
func lockAccountPair(ctx context.Context, tx pgx.Tx, accountA, accountB uuid.UUID) error {
	first, second := accountA, accountB
	if first.String() > second.String() {
		first, second = second, first
	}
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", fmt.Sprintf("social-account-pair:%s:%s", first, second))
	return err
}

func accountPairBlockedTx(ctx context.Context, tx pgx.Tx, accountA, accountB uuid.UUID) (bool, error) {
	var blocked bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM blocks
  WHERE (blocker_account_id = $1 AND blocked_account_id = $2)
     OR (blocker_account_id = $2 AND blocked_account_id = $1)
)`, accountA, accountB).Scan(&blocked)
	return blocked, err
}
