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
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = s.Pool.Query(ctx, `
SELECT space_id, profile_id, joined_at, nickname
FROM space_members
WHERE space_id = $1
ORDER BY joined_at ASC, profile_id ASC
LIMIT $2
`, spaceID, pageSize+1)
	} else {
		cursorProfile, parseErr := uuid.Parse(cursor)
		if parseErr != nil {
			return nil, "", ErrInvalidListCursor
		}
		rows, err = s.Pool.Query(ctx, `
SELECT space_id, profile_id, joined_at, nickname
FROM space_members
WHERE space_id = $1 AND profile_id > $2
ORDER BY joined_at ASC, profile_id ASC
LIMIT $3
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
	tag, err := s.Pool.Exec(ctx, `
DELETE FROM space_members WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, err = s.Pool.Exec(ctx, `
UPDATE spaces SET member_count = GREATEST(member_count - 1, 0), updated_at = now()
WHERE id = $1
`, spaceID)
	if err != nil {
		return err
	}
	_, _ = s.Pool.Exec(ctx, `
DELETE FROM space_member_timeouts WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID)
	return nil
}

// RecordMemberKicked inserts an audit_log row for a kick action.
func (s *SpaceStore) RecordMemberKicked(ctx context.Context, spaceID, profileID, actor uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_kicked', 'profile', $3, '{}')
`, spaceID, actor, profileID)
	return err
}
