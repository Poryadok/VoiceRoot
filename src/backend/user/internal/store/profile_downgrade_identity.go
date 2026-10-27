package store

import (
	"context"

	"github.com/google/uuid"
	"voice/backend/user/internal/identityassociation"
)

func (s *ProfileStore) applyDowngradeWithIdentity(ctx context.Context, account uuid.UUID, kept []uuid.UUID) error {
	tx, err := s.birthCapture.identities.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// I precedes the fresh account selection and every profile mutation. The
	// existing lifecycle trigger appends its revisions in this transaction.
	rows, err := tx.SQL().Query(ctx, `SELECT id,identity_association_id FROM profiles
	 WHERE account_id=$1 AND deleted_at IS NULL ORDER BY is_primary DESC,created_at ASC`, account)
	if err != nil {
		return err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		var association *uuid.UUID
		if err = rows.Scan(&id, &association); err != nil {
			rows.Close()
			return err
		}
		if association == nil {
			rows.Close()
			return identityassociation.ErrIncomplete
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	keptSet := make(map[uuid.UUID]struct{}, len(kept))
	for _, id := range kept {
		keptSet[id] = struct{}{}
	}
	for _, id := range ids {
		if _, keep := keptSet[id]; keep {
			_, err = tx.SQL().Exec(ctx, `UPDATE profiles SET frozen_at=NULL,updated_at=now() WHERE id=$1 AND account_id=$2`, id, account)
		} else {
			_, err = tx.SQL().Exec(ctx, `UPDATE profiles SET frozen_at=now(),updated_at=now() WHERE id=$1 AND account_id=$2 AND frozen_at IS NULL`, id, account)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
