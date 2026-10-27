package store

import (
	"context"

	"github.com/google/uuid"
	"voice/backend/user/internal/identityassociation"
)

func (s *ProfileStore) applyFreeTierFreezeWithIdentity(ctx context.Context, account uuid.UUID) error {
	tx, err := s.birthCapture.identities.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Preserve the existing primary/earliest policy, but observe it only after I.
	rows, err := tx.SQL().Query(ctx, `SELECT id,identity_association_id,frozen_at IS NOT NULL FROM profiles
	 WHERE account_id=$1 AND deleted_at IS NULL ORDER BY is_primary DESC,created_at ASC`, account)
	if err != nil {
		return err
	}
	type candidate struct {
		id     uuid.UUID
		frozen bool
	}
	profiles := []candidate{}
	for rows.Next() {
		var p candidate
		var association *uuid.UUID
		if err = rows.Scan(&p.id, &association, &p.frozen); err != nil {
			rows.Close()
			return err
		}
		if association == nil {
			rows.Close()
			return identityassociation.ErrIncomplete
		}
		profiles = append(profiles, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for i := profileCountFree; i < len(profiles); i++ {
		if profiles[i].frozen {
			continue
		}
		if _, err = tx.SQL().Exec(ctx, `UPDATE profiles SET frozen_at=now(),updated_at=now()
		 WHERE id=$1 AND account_id=$2 AND frozen_at IS NULL`, profiles[i].id, account); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
