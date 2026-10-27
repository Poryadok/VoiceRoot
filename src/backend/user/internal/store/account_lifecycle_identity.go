package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

func (s *ProfileStore) applyAccountDeletedWithIdentity(ctx context.Context, event, account uuid.UUID, occurredAt time.Time) error {
	// I must precede inbox/account/profile locks. This preserves the existing
	// legacy inactive/search projection; it does not create A4 authority.
	tx, err := s.birthCapture.identities.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = s.applyAccountDeletedTx(ctx, tx.SQL(), event, account, occurredAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
