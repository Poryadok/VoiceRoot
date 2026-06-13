package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SpaceMembersStore reads space_db.space_members for chats bound to a space.
type SpaceMembersStore struct {
	Pool *pgxpool.Pool
}

func (s *SpaceMembersStore) configured() bool {
	return s != nil && s.Pool != nil
}

// IsSpaceMember reports whether profileID belongs to spaceID in space_members.
func (s *SpaceMembersStore) IsSpaceMember(ctx context.Context, spaceID, profileID uuid.UUID) (bool, error) {
	if !s.configured() {
		return false, errors.New("space members store: pool not configured")
	}
	var one int
	err := s.Pool.QueryRow(ctx, `
SELECT 1 FROM space_members WHERE space_id = $1 AND profile_id = $2 LIMIT 1
`, spaceID, profileID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListSpaceMembers returns all space members as chat member rows (role=member).
func (s *SpaceMembersStore) ListSpaceMembers(ctx context.Context, spaceID uuid.UUID) ([]ChatMemberRow, error) {
	if !s.configured() {
		return nil, errors.New("space members store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT profile_id, joined_at
FROM space_members
WHERE space_id = $1
ORDER BY joined_at ASC, profile_id ASC
`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatMemberRow
	for rows.Next() {
		var r ChatMemberRow
		if err := rows.Scan(&r.ProfileID, &r.JoinedAt); err != nil {
			return nil, err
		}
		r.Role = "member"
		r.JoinedAt = r.JoinedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// IsEffectiveChatMember checks chat_members for standalone chats, or space_members when space_id is set.
func (s *SpaceMembersStore) IsEffectiveChatMember(ctx context.Context, dm *DMStore, row *ChatRow, profileID uuid.UUID) (bool, error) {
	if row == nil {
		return false, errors.New("chat row required")
	}
	if row.SpaceID != nil {
		if !s.configured() {
			return false, errors.New("space members store: pool not configured")
		}
		return s.IsSpaceMember(ctx, *row.SpaceID, profileID)
	}
	if dm == nil {
		return false, errors.New("dm store required")
	}
	return dm.IsChatMember(ctx, row.ID, profileID)
}

// ListEffectiveChatMembers lists members for standalone chats from chat_members, or from space_members.
func (s *SpaceMembersStore) ListEffectiveChatMembers(ctx context.Context, dm *DMStore, row *ChatRow) ([]ChatMemberRow, error) {
	if row == nil {
		return nil, errors.New("chat row required")
	}
	if row.SpaceID != nil {
		if !s.configured() {
			return nil, errors.New("space members store: pool not configured")
		}
		return s.ListSpaceMembers(ctx, *row.SpaceID)
	}
	if dm == nil {
		return nil, errors.New("dm store required")
	}
	return dm.ListChatMembers(ctx, row.ID)
}
