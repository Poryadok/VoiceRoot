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

type groupMinMembersKey struct{}

// WithGroupMinMembers lowers the add-members floor (match squad duo = 2).
func WithGroupMinMembers(ctx context.Context, min int) context.Context {
	if min < 1 {
		return ctx
	}
	return context.WithValue(ctx, groupMinMembersKey{}, min)
}

func groupAddMinMembers(ctx context.Context) int {
	if v := ctx.Value(groupMinMembersKey{}); v != nil {
		if n, ok := v.(int); ok && n > 0 {
			return n
		}
	}
	return MinGroupMembers
}

// CreateSpaceChannelChat inserts a channel chat bound to a space; members inherit from space_members.
func (s *DMStore) CreateSpaceChannelChat(ctx context.Context, creatorProfileID, spaceID uuid.UUID, name string, topic *string) (*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("channel name is required")
	}
	var chatID uuid.UUID
	var createdAt, updatedAt time.Time
	err := s.Pool.QueryRow(ctx, `
INSERT INTO chats (type, space_id, name, creator_profile_id, slow_mode_seconds, threads_enabled, allow_user_main_feed, topic)
VALUES ('channel', $1, $2, $3, 0, true, false, $4)
RETURNING id, created_at, updated_at
`, spaceID, name, creatorProfileID, optionalTopicArg(topic)).Scan(&chatID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	n := name
	sid := spaceID
	return &ChatRow{
		ID:                chatID,
		Type:              "channel",
		SpaceID:           &sid,
		Name:              &n,
		Topic:             optionalTopicPtr(topic),
		CreatorProfileID:  creatorProfileID,
		ThreadsEnabled:    true,
		AllowUserMainFeed: false,
		CreatedAt:         createdAt.UTC(),
		UpdatedAt:         updatedAt.UTC(),
	}, nil
}

// CreateSpaceGroupChat inserts a group chat bound to a space; members inherit from space_members.
func (s *DMStore) CreateSpaceGroupChat(ctx context.Context, creatorProfileID, spaceID uuid.UUID, name string, topic *string) (*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("group name is required")
	}
	var chatID uuid.UUID
	var createdAt, updatedAt time.Time
	err := s.Pool.QueryRow(ctx, `
INSERT INTO chats (type, space_id, name, creator_profile_id, slow_mode_seconds, topic)
VALUES ('group', $1, $2, $3, 0, $4)
RETURNING id, created_at, updated_at
`, spaceID, name, creatorProfileID, optionalTopicArg(topic)).Scan(&chatID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	n := name
	sid := spaceID
	return &ChatRow{
		ID:               chatID,
		Type:             "group",
		SpaceID:          &sid,
		Name:             &n,
		Topic:            optionalTopicPtr(topic),
		CreatorProfileID: creatorProfileID,
		CreatedAt:        createdAt.UTC(),
		UpdatedAt:        updatedAt.UTC(),
	}, nil
}

// CreateChannelChat inserts a standalone channel (no space_id) with the creator as owner.
func (s *DMStore) CreateChannelChat(ctx context.Context, creatorProfileID uuid.UUID, name string, topic *string) (*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("channel name is required")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var chatID uuid.UUID
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
INSERT INTO chats (type, name, creator_profile_id, slow_mode_seconds, threads_enabled, allow_user_main_feed, topic)
VALUES ('channel', $1, $2, 0, true, false, $3)
RETURNING id, created_at, updated_at
`, name, creatorProfileID, optionalTopicArg(topic)).Scan(&chatID, &createdAt, &updatedAt)
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
		ID:                chatID,
		Type:              "channel",
		Name:              &n,
		Topic:             optionalTopicPtr(topic),
		CreatorProfileID:  creatorProfileID,
		ThreadsEnabled:    true,
		AllowUserMainFeed: false,
		CreatedAt:         createdAt.UTC(),
		UpdatedAt:         updatedAt.UTC(),
		InboxBucket:       "main",
	}, nil
}

// CreateGroupChat inserts a standalone group (no space_id) with the creator as owner.
func (s *DMStore) CreateGroupChat(ctx context.Context, creatorProfileID uuid.UUID, name string, topic *string) (*ChatRow, error) {
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
INSERT INTO chats (type, name, creator_profile_id, slow_mode_seconds, topic)
VALUES ('group', $1, $2, 0, $3)
RETURNING id, created_at, updated_at
`, name, creatorProfileID, optionalTopicArg(topic)).Scan(&chatID, &createdAt, &updatedAt)
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
		Topic:            optionalTopicPtr(topic),
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
       last_message_at, created_at, updated_at, threads_enabled, allow_user_main_feed, e2e_enabled
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
	var threadsEnabled, allowUserMainFeed, e2eEnabled bool
	err := row.Scan(&id, &chatType, &spaceID, &name, &avatarURL, &topic, &creator, &slowMode,
		&lastMsg, &createdAt, &updatedAt, &threadsEnabled, &allowUserMainFeed, &e2eEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r := &ChatRow{
		ID:                id,
		Type:              chatType,
		CreatorProfileID:  creator,
		SlowModeSeconds:   slowMode,
		CreatedAt:         createdAt.UTC(),
		UpdatedAt:         updatedAt.UTC(),
		ThreadsEnabled:    threadsEnabled,
		AllowUserMainFeed: allowUserMainFeed,
		E2EEnabled:        e2eEnabled,
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
	if projected < groupAddMinMembers(ctx) {
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
	count, err := s.CountChatMembers(ctx, chatID)
	if err != nil {
		return err
	}
	if count-1 < MinGroupMembers {
		return ErrGroupTooFewMembers
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

// RemoveStandaloneGroupMember removes a member after checking the standalone
// group hierarchy in the same transaction as the delete.  The chat row is
// locked first, which serializes this with admission, role changes, and owner
// transfer; member rows are then locked in UUID order to avoid a lock cycle
// between concurrent moderation requests.
func (s *DMStore) RemoveStandaloneGroupMember(ctx context.Context, chatID, actorID, targetID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var chatType string
	var spaceID *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT type, space_id FROM chats WHERE id = $1 FOR UPDATE`, chatID).Scan(&chatType, &spaceID); err != nil {
		return err
	}
	if chatType != "group" || spaceID != nil {
		return ErrRoleChangeInvalid
	}

	rows, err := tx.Query(ctx, `
SELECT profile_id, role
FROM chat_members
WHERE chat_id = $1 AND profile_id = ANY($2::uuid[])
ORDER BY profile_id
FOR UPDATE
`, chatID, []uuid.UUID{actorID, targetID})
	if err != nil {
		return err
	}
	defer rows.Close()
	roles := make(map[uuid.UUID]string, 2)
	for rows.Next() {
		var profileID uuid.UUID
		var role string
		if err := rows.Scan(&profileID, &role); err != nil {
			return err
		}
		roles[profileID] = role
	}
	if err := rows.Err(); err != nil {
		return err
	}
	actorRole, actorOK := roles[actorID]
	targetRole, targetOK := roles[targetID]
	if !actorOK || !targetOK {
		return ErrNotGroupMember
	}
	if actorRole != "owner" && actorRole != "admin" {
		return ErrRoleChangeForbidden
	}
	if targetRole == "owner" {
		return ErrCannotRemoveOwner
	}
	if actorRole == "admin" && targetRole != "member" {
		return ErrRoleChangeForbidden
	}

	var memberCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*)::int FROM chat_members WHERE chat_id = $1`, chatID).Scan(&memberCount); err != nil {
		return err
	}
	if memberCount-1 < MinGroupMembers {
		return ErrGroupTooFewMembers
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_members WHERE chat_id = $1 AND profile_id = $2`, chatID, targetID); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
	count, err := s.CountChatMembers(ctx, chatID)
	if err != nil {
		return err
	}
	if count-1 < MinGroupMembers {
		return ErrGroupTooFewMembers
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

// SetStandaloneGroupMemberRole mutates a standalone group membership role while
// holding the chat and both membership rows. This serializes concurrent role
// changes with ownership transfers.
func (s *DMStore) SetStandaloneGroupMemberRole(ctx context.Context, chatID, actorID, targetID uuid.UUID, role string) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	if role != "admin" && role != "member" {
		return ErrRoleChangeInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var chatType string
	var spaceID *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT type, space_id FROM chats WHERE id = $1 FOR UPDATE`, chatID).Scan(&chatType, &spaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgx.ErrNoRows
		}
		return err
	}
	if chatType != "group" || spaceID != nil {
		return ErrRoleChangeInvalid
	}

	roles, err := lockGroupMemberRoles(ctx, tx, chatID, actorID, targetID)
	if err != nil {
		return err
	}
	actorRole, actorOK := roles[actorID]
	targetRole, targetOK := roles[targetID]
	if !actorOK || !targetOK {
		return ErrNotGroupMember
	}
	if targetRole == "owner" {
		return ErrRoleChangeInvalid
	}
	if role == "admin" {
		if targetRole != "member" {
			return ErrRoleChangeInvalid
		}
		if actorRole != "owner" && actorRole != "admin" {
			return ErrRoleChangeForbidden
		}
	} else {
		if targetRole != "admin" {
			return ErrRoleChangeInvalid
		}
		if actorRole != "owner" {
			return ErrRoleChangeForbidden
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE chat_members SET role = $3 WHERE chat_id = $1 AND profile_id = $2`, chatID, targetID, role); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// TransferGroupOwnership moves the owner role to another member.
func (s *DMStore) TransferGroupOwnership(ctx context.Context, chatID, ownerID, newOwnerID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	if ownerID == newOwnerID {
		return errors.New("new owner must differ from current owner")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var chatType string
	var spaceID *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT type, space_id FROM chats WHERE id = $1 FOR UPDATE`, chatID).Scan(&chatType, &spaceID); err != nil {
		return err
	}
	if chatType != "group" || spaceID != nil {
		return ErrNotGroupOwner
	}
	roles, err := lockGroupMemberRoles(ctx, tx, chatID, ownerID, newOwnerID)
	if err != nil {
		return err
	}
	ownerRole, ownerOK := roles[ownerID]
	newOwnerRole, newOwnerOK := roles[newOwnerID]
	if !ownerOK || !newOwnerOK {
		return pgx.ErrNoRows
	}
	if ownerRole != "owner" {
		return ErrNotGroupOwner
	}
	_ = newOwnerRole
	if _, err := tx.Exec(ctx, `UPDATE chat_members SET role = 'member' WHERE chat_id = $1 AND profile_id = $2`, chatID, ownerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE chat_members SET role = 'owner' WHERE chat_id = $1 AND profile_id = $2`, chatID, newOwnerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// lockGroupMemberRoles takes membership locks in a deterministic order.  All
// standalone role mutations call it only after locking the chat row.
func lockGroupMemberRoles(ctx context.Context, tx pgx.Tx, chatID uuid.UUID, ids ...uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := tx.Query(ctx, `
SELECT profile_id, role
FROM chat_members
WHERE chat_id = $1 AND profile_id = ANY($2::uuid[])
ORDER BY profile_id
FOR UPDATE
`, chatID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := make(map[uuid.UUID]string, len(ids))
	for rows.Next() {
		var profileID uuid.UUID
		var role string
		if err := rows.Scan(&profileID, &role); err != nil {
			return nil, err
		}
		roles[profileID] = role
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}

// UpdateGroupChat updates mutable group/channel fields.
func (s *DMStore) UpdateGroupChat(ctx context.Context, chatID uuid.UUID, name, avatarURL, topic *string, slowModeSeconds *int32, threadsEnabled, allowUserMainFeed *bool) (*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if name == nil && avatarURL == nil && topic == nil && slowModeSeconds == nil && threadsEnabled == nil && allowUserMainFeed == nil {
		return s.FindChatByID(ctx, chatID)
	}
	sets := make([]string, 0, 8)
	args := make([]any, 0, 9)
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
	if topic != nil {
		sets = append(sets, fmt.Sprintf("topic = $%d", argN))
		args = append(args, optionalTopicArg(topic))
		argN++
	}
	if slowModeSeconds != nil {
		sets = append(sets, fmt.Sprintf("slow_mode_seconds = $%d", argN))
		args = append(args, *slowModeSeconds)
		argN++
	}
	if threadsEnabled != nil {
		sets = append(sets, fmt.Sprintf("threads_enabled = $%d", argN))
		args = append(args, *threadsEnabled)
		argN++
	}
	if allowUserMainFeed != nil {
		sets = append(sets, fmt.Sprintf("allow_user_main_feed = $%d", argN))
		args = append(args, *allowUserMainFeed)
		argN++
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, chatID)
	q := fmt.Sprintf(`
UPDATE chats
SET %s
WHERE id = $%d AND type IN ('group', 'channel')
RETURNING id, type, space_id, name, avatar_url, topic, creator_profile_id, slow_mode_seconds,
          last_message_at, created_at, updated_at, threads_enabled, allow_user_main_feed, e2e_enabled
`, strings.Join(sets, ", "), argN)
	return scanChatRow(s.Pool.QueryRow(ctx, q, args...))
}

func optionalTopicArg(topic *string) any {
	if topic == nil {
		return nil
	}
	t := strings.TrimSpace(*topic)
	if t == "" {
		return nil
	}
	return t
}

func optionalTopicPtr(topic *string) *string {
	if topic == nil {
		return nil
	}
	t := strings.TrimSpace(*topic)
	if t == "" {
		return nil
	}
	return &t
}

var (
	ErrGroupTooFewMembers  = errors.New("group must have at least 3 members")
	ErrGroupMemberLimit    = errors.New("group member limit is 500")
	ErrCannotRemoveOwner   = errors.New("cannot remove group owner")
	ErrNotGroupOwner       = errors.New("caller is not group owner")
	ErrOwnerMustTransfer   = errors.New("group owner must transfer ownership before leaving")
	ErrNotGroupMember      = errors.New("group member not found")
	ErrRoleChangeForbidden = errors.New("caller cannot make this group role change")
	ErrRoleChangeInvalid   = errors.New("invalid group role change")
)
