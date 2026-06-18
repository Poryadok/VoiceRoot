package store

import (
	"context"

	"github.com/google/uuid"
)

// AreCoMembers reports whether two profiles share at least one space membership.
// When spaceIDs is non-empty, only those spaces are considered.
func (s *SpaceStore) AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, nil
	}
	if profileA == profileB {
		return true, nil
	}
	var exists bool
	var err error
	if len(spaceIDs) == 0 {
		err = s.Pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM space_members m1
  JOIN space_members m2 ON m1.space_id = m2.space_id
  WHERE m1.profile_id = $1 AND m2.profile_id = $2
)`, profileA, profileB).Scan(&exists)
	} else {
		err = s.Pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM space_members m1
  JOIN space_members m2 ON m1.space_id = m2.space_id
  WHERE m1.profile_id = $1 AND m2.profile_id = $2
    AND m1.space_id = ANY($3::uuid[])
)`, profileA, profileB, spaceIDs).Scan(&exists)
	}
	if err != nil {
		return false, err
	}
	return exists, nil
}
