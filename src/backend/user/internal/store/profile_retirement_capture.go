package store

import (
	"context"

	"github.com/google/uuid"
	"voice/backend/user/internal/identityassociation"
)

func (s *ProfileStore) softDeleteProfileWithIdentityCapture(ctx context.Context, account, profile uuid.UUID) error {
	// Acquire I before any profile lock or fresh observation. User22's existing
	// lifecycle trigger owns the journal revision for the soft archive.
	tx, err := s.birthCapture.identities.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var association *uuid.UUID
	err = tx.SQL().QueryRow(ctx, `SELECT identity_association_id FROM profiles
	 WHERE id=$1 AND account_id=$2 AND is_primary=false AND deleted_at IS NULL FOR UPDATE`, profile, account).Scan(&association)
	if err != nil {
		return err
	}
	if association == nil {
		// A legacy row is not silently represented as captured history.
		return identityassociation.ErrIncomplete
	}
	if err = s.softDeleteProfileTx(ctx, tx.SQL(), account, profile); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
