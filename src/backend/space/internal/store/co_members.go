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
	return s.areCoMembers(ctx, profileA, profileB, spaceIDs, nil)
}

// AreCoMembersWithAccountBans reports shared membership only when neither
// trusted account is banned from the matching Space. Account IDs must come
// from the User-owned profile mapping, never from the request payload.
func (s *SpaceStore) AreCoMembersWithAccountBans(ctx context.Context, profileA, profileB uuid.UUID, accountIDs []uuid.UUID, spaceIDs []uuid.UUID) (bool, error) {
	return s.areCoMembers(ctx, profileA, profileB, spaceIDs, accountIDs)
}

func (s *SpaceStore) areCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs, accountIDs []uuid.UUID) (bool, error) {
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
			err := scoped.db().QueryRow(ctx, `WITH effective AS (
				SELECT space_id,profile_id FROM space_members
				UNION SELECT m.space_id,m.profile_id FROM community_roster_members m JOIN community_owner_authority a
				ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation AND a.status='active'
				WHERE m.revoked_at IS NULL AND m.lease_expires_at>clock_timestamp() AND a.roster_lease_expires_at>clock_timestamp()
			) SELECT EXISTS (SELECT 1 FROM effective m1 JOIN effective m2 ON m1.space_id=m2.space_id
				WHERE m1.profile_id=$1 AND m2.profile_id=$2 AND m1.space_id=ANY($3::uuid[])
				AND NOT EXISTS (SELECT 1 FROM space_bans b WHERE b.space_id=m1.space_id AND b.account_id=ANY($4::uuid[])))`,
				profileA, profileB, normalizedOwnershipSpaceIDs(spaceIDs), accountIDs).Scan(&exists)
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
		candidates, err := sharedMembershipSpaceIDs(ctx, tx, profileA, profileB, accountIDs)
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
		var frozen, sharedLive bool
		err = tx.QueryRow(ctx, `WITH effective AS (
			SELECT space_id,profile_id FROM space_members
			UNION SELECT m.space_id,m.profile_id FROM community_roster_members m JOIN community_owner_authority a
			ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation AND a.status='active'
			WHERE m.revoked_at IS NULL AND m.lease_expires_at>clock_timestamp() AND a.roster_lease_expires_at>clock_timestamp()
		), candidate AS (
			SELECT m1.space_id FROM effective m1 JOIN effective m2 ON m1.space_id=m2.space_id
			WHERE m1.profile_id=$1 AND m2.profile_id=$2
			AND NOT EXISTS (SELECT 1 FROM space_bans b WHERE b.space_id=m1.space_id AND b.account_id=ANY($3::uuid[]))
		) SELECT COALESCE(array_agg(space_id ORDER BY space_id),'{}'::uuid[]),
			EXISTS(SELECT 1 FROM ownership_journal j JOIN candidate c ON c.space_id=j.space_id WHERE j.state NOT IN ('completed','aborted')),
			EXISTS(SELECT 1 FROM candidate c WHERE NOT EXISTS (
				SELECT 1 FROM space_lifecycle_aggregates a WHERE a.space_id=c.space_id AND a.phase <> 'LIVE'))
			FROM candidate`, profileA, profileB, accountIDs).Scan(&verified, &frozen, &sharedLive)
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
		return sharedLive, nil
	}
	return false, ErrOwnershipFrozen
}

func sharedMembershipSpaceIDs(ctx context.Context, tx pgx.Tx, profileA, profileB uuid.UUID, accountIDs []uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `WITH effective AS (
		SELECT space_id,profile_id FROM space_members
		UNION SELECT m.space_id,m.profile_id FROM community_roster_members m JOIN community_owner_authority a
		ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation AND a.status='active'
		WHERE m.revoked_at IS NULL AND m.lease_expires_at>clock_timestamp() AND a.roster_lease_expires_at>clock_timestamp()
	) SELECT m1.space_id FROM effective m1 JOIN effective m2 ON m1.space_id=m2.space_id
		WHERE m1.profile_id=$1 AND m2.profile_id=$2
		AND NOT EXISTS (SELECT 1 FROM space_bans b WHERE b.space_id=m1.space_id AND b.account_id=ANY($3::uuid[]))
		ORDER BY m1.space_id`, profileA, profileB, accountIDs)
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
