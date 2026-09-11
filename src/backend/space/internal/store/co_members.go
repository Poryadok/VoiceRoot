package store

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AreCoMembers reports whether two profiles share at least one space membership.
// When spaceIDs is non-empty, only those spaces are considered.
func (s *SpaceStore) AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, nil
	}
	if profileA == profileB && len(spaceIDs) == 0 {
		return true, nil
	}
	if len(spaceIDs) != 0 {
		return withOwnershipScopeValue(s, ctx, spaceIDs, func(scoped *SpaceStore) (bool, error) {
			if profileA == profileB {
				return true, nil
			}
			var exists bool
			err := scoped.db().QueryRow(ctx, `SELECT EXISTS (
				SELECT 1 FROM space_members m1 JOIN space_members m2 ON m1.space_id=m2.space_id
				WHERE m1.profile_id=$1 AND m2.profile_id=$2 AND m1.space_id=ANY($3::uuid[])
			)`, profileA, profileB, normalizedOwnershipSpaceIDs(spaceIDs)).Scan(&exists)
			return exists, err
		})
	}

	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return false, err
		}
		rollback := func() {
			cleanupCtx, cancel := BoundedCleanupContext(ctx)
			defer cancel()
			_ = tx.Rollback(cleanupCtx)
		}
		candidates, err := sharedMembershipSpaceIDs(ctx, tx, profileA, profileB)
		if err != nil {
			rollback()
			return false, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
		}
		for _, id := range candidates {
			if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(id)); err != nil {
				rollback()
				return false, err
			}
		}
		var verified []uuid.UUID
		var frozen bool
		err = tx.QueryRow(ctx, `WITH candidate AS (
			SELECT m1.space_id FROM space_members m1 JOIN space_members m2 ON m1.space_id=m2.space_id
			WHERE m1.profile_id=$1 AND m2.profile_id=$2
		) SELECT COALESCE(array_agg(space_id ORDER BY space_id),'{}'::uuid[]),
			EXISTS(SELECT 1 FROM ownership_journal j JOIN candidate c ON c.space_id=j.space_id WHERE j.state NOT IN ('completed','aborted'))
			FROM candidate`, profileA, profileB).Scan(&verified, &frozen)
		if err != nil {
			rollback()
			return false, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
		}
		verified = normalizedOwnershipSpaceIDs(verified)
		if !slices.Equal(candidates, verified) {
			rollback()
			continue
		}
		if frozen {
			rollback()
			return false, ErrOwnershipFrozen
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return len(verified) != 0, nil
	}
	return false, ErrOwnershipFrozen
}

func sharedMembershipSpaceIDs(ctx context.Context, tx pgx.Tx, profileA, profileB uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `SELECT m1.space_id FROM space_members m1 JOIN space_members m2 ON m1.space_id=m2.space_id
		WHERE m1.profile_id=$1 AND m2.profile_id=$2 ORDER BY m1.space_id`, profileA, profileB)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return normalizedOwnershipSpaceIDs(ids), nil
}
