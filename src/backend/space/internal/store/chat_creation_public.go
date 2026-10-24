package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CanDiscloseChatCreation evaluates the existing membership/guest contract under
// one Space mutation scope. Separate locks could combine an old membership with
// a newer guest opt-in that never authorized that member at the same instant.
func (s *SpaceStore) CanDiscloseChatCreation(ctx context.Context, space, profile uuid.UUID, guest bool) (bool, error) {
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{space}, func(scoped *SpaceStore) (bool, error) {
		member, err := scoped.IsSpaceMember(ctx, space, profile)
		if err != nil || !member {
			return false, err
		}
		row, err := scoped.GetSpace(ctx, space)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return row != nil && (!guest || row.AllowGuests), nil
	})
}
