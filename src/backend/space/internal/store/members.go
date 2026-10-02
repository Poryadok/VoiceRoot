package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrMemberNotFound = errors.New("member not found")

// ListSpaceMembersPage lists members of a space with cursor pagination.
func (s *SpaceStore) ListSpaceMembersPage(ctx context.Context, spaceID uuid.UUID, pageSize int32, cursor string) ([]*MembershipRow, string, error) {
	if s == nil || s.Pool == nil {
		return nil, "", errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		type result struct {
			rows   []*MembershipRow
			cursor string
		}
		value, err := withOwnershipScopeValue(s, ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) (result, error) {
			rows, next, callErr := scoped.ListSpaceMembersPage(ctx, spaceID, pageSize, cursor)
			return result{rows: rows, cursor: next}, callErr
		})
		return value.rows, value.cursor, err
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = s.db().Query(ctx, `
WITH effective_members AS (
  SELECT space_id,profile_id,joined_at,nickname FROM space_members WHERE space_id=$1
  UNION ALL
  SELECT m.space_id,m.profile_id,o.created_at,NULL::text FROM community_roster_members m
  JOIN community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation AND a.status='active'
  JOIN community_roster_operations o ON o.space_id=m.space_id AND o.owner_generation=m.owner_generation AND o.source_revision=m.source_revision
  WHERE m.space_id=$1 AND m.revoked_at IS NULL AND m.lease_expires_at>clock_timestamp()
    AND a.roster_lease_expires_at>clock_timestamp()
    AND NOT EXISTS (SELECT 1 FROM space_members sm WHERE sm.space_id=m.space_id AND sm.profile_id=m.profile_id)
)
SELECT space_id,profile_id,joined_at,nickname FROM effective_members
ORDER BY joined_at ASC,profile_id ASC LIMIT $2
`, spaceID, pageSize+1)
	} else {
		cursorProfile, parseErr := uuid.Parse(cursor)
		if parseErr != nil {
			return nil, "", ErrInvalidListCursor
		}
		rows, err = s.db().Query(ctx, `
WITH effective_members AS (
  SELECT space_id,profile_id,joined_at,nickname FROM space_members WHERE space_id=$1
  UNION ALL
  SELECT m.space_id,m.profile_id,o.created_at,NULL::text FROM community_roster_members m
  JOIN community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation AND a.status='active'
  JOIN community_roster_operations o ON o.space_id=m.space_id AND o.owner_generation=m.owner_generation AND o.source_revision=m.source_revision
  WHERE m.space_id=$1 AND m.revoked_at IS NULL AND m.lease_expires_at>clock_timestamp()
    AND a.roster_lease_expires_at>clock_timestamp()
    AND NOT EXISTS (SELECT 1 FROM space_members sm WHERE sm.space_id=m.space_id AND sm.profile_id=m.profile_id)
)
SELECT space_id,profile_id,joined_at,nickname FROM effective_members WHERE profile_id>$2
ORDER BY joined_at ASC,profile_id ASC LIMIT $3
`, spaceID, cursorProfile, pageSize+1)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []*MembershipRow
	for rows.Next() {
		r, err := scanMembershipRow(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	var next string
	if len(out) > int(pageSize) {
		next = out[pageSize-1].ProfileID.String()
		out = out[:pageSize]
	}
	return out, next, nil
}

// RemoveMember deletes membership and decrements member_count.
func (s *SpaceStore) RemoveMember(ctx context.Context, spaceID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return s.withOwnershipScope(ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) error {
			return scoped.RemoveMember(ctx, spaceID, profileID)
		})
	}
	tag, err := s.db().Exec(ctx, `
DELETE FROM space_members WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, err = s.db().Exec(ctx, `
UPDATE spaces SET member_count = GREATEST(member_count - 1, 0), updated_at = now()
WHERE id = $1
`, spaceID)
	if err != nil {
		return err
	}
	_, _ = s.db().Exec(ctx, `
DELETE FROM space_member_timeouts WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID)
	return nil
}

// KickMember removes membership and records the successful administrative
// effect in the same transaction. A failed audit write rolls back the removal.
func (s *SpaceStore) KickMember(ctx context.Context, spaceID, profileID, actor uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return s.withOwnershipScope(ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) error {
			return scoped.KickMember(ctx, spaceID, profileID, actor)
		})
	}
	tag, err := s.db().Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, spaceID, profileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if _, err := s.db().Exec(ctx, `UPDATE spaces SET member_count=GREATEST(member_count-1,0),updated_at=now() WHERE id=$1`, spaceID); err != nil {
		return err
	}
	if _, err := s.db().Exec(ctx, `DELETE FROM space_member_timeouts WHERE space_id=$1 AND profile_id=$2`, spaceID, profileID); err != nil {
		return err
	}
	_, err = s.db().Exec(ctx, `
INSERT INTO audit_log(space_id,actor_profile_id,action,target_type,target_id,details)
VALUES($1,$2,'member_kicked','profile',$3,'{}')`, spaceID, actor, profileID)
	return err
}

// RecordMemberKicked inserts an audit_log row for a kick action.
func (s *SpaceStore) RecordMemberKicked(ctx context.Context, spaceID, profileID, actor uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return s.withOwnershipScope(ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) error {
			return scoped.RecordMemberKicked(ctx, spaceID, profileID, actor)
		})
	}
	_, err := s.db().Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_kicked', 'profile', $3, '{}')
`, spaceID, actor, profileID)
	return err
}
