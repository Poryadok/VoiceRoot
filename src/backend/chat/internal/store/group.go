package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	GroupMemberLimit = 500
	MinGroupMembers  = 3
)

// CreateGroupChat inserts a standalone group (no space_id) with the creator as owner.
func (s *DMStore) CreateGroupChat(ctx context.Context, creatorProfileID uuid.UUID, name string) (*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("group name is required")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var chatID uuid.UUID
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
INSERT INTO chats (type, name, creator_profile_id, slow_mode_seconds)
VALUES ('group', $1, $2, 0)
RETURNING id, created_at, updated_at
`, name, creatorProfileID).Scan(&chatID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket)
VALUES ($1, $2, 'owner', 'main')
`, chatID, creatorProfileID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	n := name
	return &ChatRow{
		ID:               chatID,
		Type:             "group",
		Name:             &n,
		CreatorProfileID: creatorProfileID,
		CreatedAt:        createdAt.UTC(),
		UpdatedAt:        updatedAt.UTC(),
		InboxBucket:      "main",
	}, nil
}

// FindChatByID loads any chat row by id.
func (s *DMStore) FindChatByID(ctx context.Context, chatID uuid.UUID) (*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	return scanChatRow(s.Pool.QueryRow(ctx, `
SELECT id, type, space_id, name, avatar_url, topic, creator_profile_id, slow_mode_seconds,
       last_message_at, created_at, updated_at
FROM chats
WHERE id = $1
`, chatID))
}

func scanChatRow(row pgx.Row) (*ChatRow, error) {
	var id, creator uuid.UUID
	var chatType string
	var spaceID sql.NullString
	var name, avatarURL, topic sql.NullString
	var slowMode int32
	var lastMsg sql.NullTime
	var createdAt, updatedAt time.Time
	err := row.Scan(&id, &chatType, &spaceID, &name, &avatarURL, &topic, &creator, &slowMode,
		&lastMsg, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r := &ChatRow{
		ID:               id,
		Type:             chatType,
		CreatorProfileID: creator,
		SlowModeSeconds:  slowMode,
		CreatedAt:        createdAt.UTC(),
		UpdatedAt:        updatedAt.UTC(),
	}
	if spaceID.Valid {
		if sid, perr := uuid.Parse(spaceID.String); perr == nil {
			r.SpaceID = &sid
		}
	}
	if name.Valid {
		n := name.String
		r.Name = &n
	}
	if avatarURL.Valid {
		a := avatarURL.String
		r.AvatarURL = &a
	}
	if topic.Valid {
		t := topic.String
		r.Topic = &t
	}
	if lastMsg.Valid {
		t := lastMsg.Time.UTC()
		r.LastMessageAt = &t
	}
	return r, nil
}

// CountChatMembers returns the number of members in a chat.
func (s *DMStore) CountChatMembers(ctx context.Context, chatID uuid.UUID) (int, error) {
	if s == nil || s.Pool == nil {
		return 0, errors.New("dm store: pool not configured")
	}
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM chat_members WHERE chat_id = $1`, chatID).Scan(&n)
	return n, err
}

// GetMemberRole returns the member role or empty string when not a member.
func (s *DMStore) GetMemberRole(ctx context.Context, chatID, profileID uuid.UUID) (string, error) {
	if s == nil || s.Pool == nil {
		return "", errors.New("dm store: pool not configured")
	}
	var role string
	err := s.Pool.QueryRow(ctx, `
SELECT role FROM chat_members WHERE chat_id = $1 AND profile_id = $2
`, chatID, profileID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// AddGroupMembers inserts new members into a standalone group chat.
func (s *DMStore) AddGroupMembers(ctx context.Context, chatID uuid.UUID, profileIDs []uuid.UUID) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if len(profileIDs) == 0 {
		return nil, errors.New("profile_ids is required")
	}
	unique := make([]uuid.UUID, 0, len(profileIDs))
	seen := make(map[uuid.UUID]struct{}, len(profileIDs))
	for _, id := range profileIDs {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, errors.New("profile_ids is required")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var chatType string
	err = tx.QueryRow(ctx, `SELECT type FROM chats WHERE id = $1 FOR UPDATE`, chatID).Scan(&chatType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	if chatType != "group" {
		return nil, fmt.Errorf("add members only supported for group chats")
	}

	var current int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*)::int FROM chat_members WHERE chat_id = $1`, chatID).Scan(&current); err != nil {
		return nil, err
	}

	var added []uuid.UUID
	for _, pid := range unique {
		var exists int
		err := tx.QueryRow(ctx, `
SELECT 1 FROM chat_members WHERE chat_id = $1 AND profile_id = $2
`, chatID, pid).Scan(&exists)
		if err == nil {
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		added = append(added, pid)
	}

	projected := current + len(added)
	if projected < MinGroupMembers {
		return nil, ErrGroupTooFewMembers
	}
	if projected > GroupMemberLimit {
		return nil, ErrGroupMemberLimit
	}

	for _, pid := range added {
		if _, err := tx.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket)
VALUES ($1, $2, 'member', 'main')
`, chatID, pid); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return added, nil
}

// RemoveGroupMember deletes membership from a group chat. The owner cannot be removed.
func (s *DMStore) RemoveGroupMember(ctx context.Context, chatID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	role, err := s.GetMemberRole(ctx, chatID, profileID)
	if err != nil {
		return err
	}
	if role == "" {
		return pgx.ErrNoRows
	}
	if role == "owner" {
		return ErrCannotRemoveOwner
	}
	ct, err := s.Pool.Exec(ctx, `
DELETE FROM chat_members m
USING chats c
WHERE m.chat_id = c.id AND c.type = 'group'
  AND m.chat_id = $1 AND m.profile_id = $2
`, chatID, profileID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// LeaveGroupChat removes the caller from a standalone group. Owners must transfer ownership first.
func (s *DMStore) LeaveGroupChat(ctx context.Context, chatID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	role, err := s.GetMemberRole(ctx, chatID, profileID)
	if err != nil {
		return err
	}
	if role == "" {
		return pgx.ErrNoRows
	}
	if role == "owner" {
		return ErrOwnerMustTransfer
	}
	ct, err := s.Pool.Exec(ctx, `
DELETE FROM chat_members m
USING chats c
WHERE m.chat_id = c.id AND c.type = 'group'
  AND m.chat_id = $1 AND m.profile_id = $2
`, chatID, profileID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateGroupChat updates mutable group fields (name, avatar_url).
func (s *DMStore) UpdateGroupChat(ctx context.Context, chatID uuid.UUID, name, avatarURL *string) (*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if name == nil && avatarURL == nil {
		return s.FindChatByID(ctx, chatID)
	}
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)
	argN := 1
	if name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argN))
		args = append(args, strings.TrimSpace(*name))
		argN++
	}
	if avatarURL != nil {
		sets = append(sets, fmt.Sprintf("avatar_url = $%d", argN))
		args = append(args, *avatarURL)
		argN++
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, chatID)
	q := fmt.Sprintf(`
UPDATE chats
SET %s
WHERE id = $%d AND type = 'group'
RETURNING id, type, space_id, name, avatar_url, topic, creator_profile_id, slow_mode_seconds,
          last_message_at, created_at, updated_at
`, strings.Join(sets, ", "), argN)
	return scanChatRow(s.Pool.QueryRow(ctx, q, args...))
}

var (
	ErrGroupTooFewMembers = errors.New("group must have at least 3 members")
	ErrGroupMemberLimit   = errors.New("group member limit is 500")
	ErrCannotRemoveOwner  = errors.New("cannot remove group owner")
	ErrOwnerMustTransfer  = errors.New("group owner must transfer ownership before leaving")
)
