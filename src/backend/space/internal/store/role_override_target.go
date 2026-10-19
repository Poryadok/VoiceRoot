package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ValidateRoleOverrideTarget serializes current local lifecycle/journal and
// resource existence under the ordinary Space fence. No ACL RPC is performed.
func (s *SpaceStore) ValidateRoleOverrideTarget(ctx context.Context, spaceID uuid.UUID, roomID *uuid.UUID) error {
	return s.withOwnershipScope(ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) error {
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1)`
		args := []any{spaceID}
		if roomID != nil {
			query = `SELECT EXISTS(SELECT 1 FROM spaces s JOIN voice_rooms v ON v.space_id=s.id WHERE s.id=$1 AND v.id=$2)`
			args = append(args, *roomID)
		}
		if err := scoped.db().QueryRow(ctx, query, args...).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}
		return nil
	})
}
