package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// DMPeerDeletionTarget identifies the still-active side of a direct DM whose
// other participant belongs to a deleted profile set.
type DMPeerDeletionTarget struct {
	ChatID             uuid.UUID
	SurvivingProfileID uuid.UUID
}

// ListDMPeerDeletionTargets returns one target for every direct DM that has at
// least one deleted participant and exactly one surviving participant. The
// input is treated as a set, and rows use canonical UUID-text ordering so the
// result is stable regardless of input order.
func (s *DMStore) ListDMPeerDeletionTargets(ctx context.Context, deletedProfileIDs []uuid.UUID) ([]DMPeerDeletionTarget, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if len(deletedProfileIDs) == 0 {
		return []DMPeerDeletionTarget{}, nil
	}

	rows, err := s.Pool.Query(ctx, `
WITH deleted_profiles AS (
  SELECT DISTINCT profile_id
  FROM unnest($1::uuid[]) AS deleted(profile_id)
), dm_members AS (
  SELECT
    c.id AS chat_id,
    array_agg(m.profile_id) FILTER (WHERE d.profile_id IS NULL) AS surviving_profile_ids,
    COUNT(*) FILTER (WHERE d.profile_id IS NOT NULL) AS deleted_member_count,
    COUNT(*) FILTER (WHERE d.profile_id IS NULL) AS surviving_member_count
  FROM chats c
  INNER JOIN chat_members m ON m.chat_id = c.id
  LEFT JOIN deleted_profiles d ON d.profile_id = m.profile_id
  WHERE c.type = 'dm'
  GROUP BY c.id
)
SELECT chat_id, surviving_profile_ids[1]
FROM dm_members
WHERE deleted_member_count > 0
  AND surviving_member_count = 1
ORDER BY chat_id::text ASC, surviving_profile_ids[1]::text ASC
`, deletedProfileIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make([]DMPeerDeletionTarget, 0)
	for rows.Next() {
		var target DMPeerDeletionTarget
		if err := rows.Scan(&target.ChatID, &target.SurvivingProfileID); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return targets, nil
}
