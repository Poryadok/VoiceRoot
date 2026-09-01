package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrFolderNotFound is returned when a folder id does not belong to the caller profile.
var ErrFolderNotFound = errors.New("folder not found")

// ErrSystemFolderMembership is returned when mutating explicit membership on a system folder.
var ErrSystemFolderMembership = errors.New("system folders use implicit membership")

// ErrFolderChatNotMember is returned when a chat is not in a custom folder.
var ErrFolderChatNotMember = errors.New("chat not in folder")

// ErrFolderChatPredicate is returned when a chat does not match a system folder predicate.
var ErrFolderChatPredicate = errors.New("chat does not match folder predicate")

type folderFilterConfig struct {
	SystemKey  string `json:"system_key"`
	ChatType   string `json:"chat_type"`
	HasSpaceID bool   `json:"has_space_id"`
}

// GetFolder returns one folder owned by profileID.
func (s *DMStore) GetFolder(ctx context.Context, profileID, folderID uuid.UUID) (*FolderRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	var row FolderRow
	err := s.Pool.QueryRow(ctx, `
SELECT id, profile_id, name, folder_type, filter_config_json::text, sort_order, created_at, updated_at
FROM folders
WHERE id = $1 AND profile_id = $2
`, folderID, profileID).Scan(
		&row.ID, &row.ProfileID, &row.Name, &row.FolderType, &row.FilterConfigJSON, &row.SortOrder, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFolderNotFound
		}
		return nil, err
	}
	return &row, nil
}

func parseFolderFilterConfig(raw string) folderFilterConfig {
	var cfg folderFilterConfig
	_ = json.Unmarshal([]byte(raw), &cfg)
	return cfg
}

func chatMatchesFolderPredicate(chatType string, spaceID *uuid.UUID, cfg folderFilterConfig) bool {
	switch cfg.SystemKey {
	case "all", "":
		if cfg.ChatType != "" && chatType != cfg.ChatType {
			return false
		}
		if cfg.HasSpaceID && spaceID == nil {
			return false
		}
		return true
	case "dm", "group", "channel":
		return chatType == cfg.ChatType
	case "spaces":
		return spaceID != nil
	default:
		if cfg.ChatType != "" && chatType != cfg.ChatType {
			return false
		}
		if cfg.HasSpaceID && spaceID == nil {
			return false
		}
		return true
	}
}

func (s *DMStore) memberChatRow(ctx context.Context, profileID, chatID uuid.UUID) (*ChatRow, error) {
	var archived bool
	var chatType string
	var spaceID *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
SELECT c.type, c.space_id, COALESCE(m.is_archived, false)
FROM chats c
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $2
WHERE c.id = $1
`, chatID, profileID).Scan(&chatType, &spaceID, &archived)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	if archived {
		return nil, fmt.Errorf("archived chat cannot be added to folder")
	}
	return &ChatRow{ID: chatID, Type: chatType, SpaceID: spaceID}, nil
}

// AddChatToFolder adds explicit membership for a custom folder.
func (s *DMStore) AddChatToFolder(ctx context.Context, profileID, folderID, chatID uuid.UUID, sortOrder *int32) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	folder, err := s.GetFolder(ctx, profileID, folderID)
	if err != nil {
		return err
	}
	if folder.FolderType != "custom" {
		return ErrSystemFolderMembership
	}
	row, err := s.memberChatRow(ctx, profileID, chatID)
	if err != nil {
		return err
	}
	_ = row

	var nextOrder int32
	if err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(MAX(sort_order), -1) + 1 FROM folder_chats WHERE profile_id = $1 AND folder_id = $2
`, profileID, folderID).Scan(&nextOrder); err != nil {
		return err
	}
	order := nextOrder
	if sortOrder != nil {
		order = *sortOrder
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO folder_chats (profile_id, folder_id, chat_id, sort_order)
VALUES ($1, $2, $3, $4)
ON CONFLICT (profile_id, folder_id, chat_id) DO UPDATE SET sort_order = EXCLUDED.sort_order
`, profileID, folderID, chatID, order)
	return err
}

// RemoveChatFromFolder removes explicit membership from a custom folder.
func (s *DMStore) RemoveChatFromFolder(ctx context.Context, profileID, folderID, chatID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	folder, err := s.GetFolder(ctx, profileID, folderID)
	if err != nil {
		return err
	}
	if folder.FolderType != "custom" {
		return ErrSystemFolderMembership
	}
	_, err = s.Pool.Exec(ctx, `
DELETE FROM folder_chats WHERE profile_id = $1 AND folder_id = $2 AND chat_id = $3
`, profileID, folderID, chatID)
	return err
}

// ReorderFolderChats replaces manual order for a custom folder.
func (s *DMStore) ReorderFolderChats(ctx context.Context, profileID, folderID uuid.UUID, chatIDs []uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	folder, err := s.GetFolder(ctx, profileID, folderID)
	if err != nil {
		return err
	}
	if folder.FolderType != "custom" {
		return ErrSystemFolderMembership
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for i, chatID := range chatIDs {
		ct, err := tx.Exec(ctx, `
UPDATE folder_chats
SET sort_order = $4
WHERE profile_id = $1 AND folder_id = $2 AND chat_id = $3
`, profileID, folderID, chatID, int32(i))
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrFolderChatNotMember
		}
	}
	return tx.Commit(ctx)
}

// PinChatInFolder pins a chat within a folder (custom membership or system overlay).
func (s *DMStore) PinChatInFolder(ctx context.Context, profileID, folderID, chatID uuid.UUID, pinOrder *int32) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	folder, err := s.GetFolder(ctx, profileID, folderID)
	if err != nil {
		return err
	}
	row, err := s.memberChatRow(ctx, profileID, chatID)
	if err != nil {
		return err
	}
	cfg := parseFolderFilterConfig(folder.FilterConfigJSON)
	if folder.FolderType == "custom" {
		var exists int
		err = s.Pool.QueryRow(ctx, `
SELECT 1 FROM folder_chats WHERE profile_id = $1 AND folder_id = $2 AND chat_id = $3
`, profileID, folderID, chatID).Scan(&exists)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrFolderChatNotMember
			}
			return err
		}
	} else if !chatMatchesFolderPredicate(row.Type, row.SpaceID, cfg) {
		return ErrFolderChatPredicate
	}

	var nextPin int32
	if err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(MAX(pin_order), -1) + 1 FROM folder_chats
WHERE profile_id = $1 AND folder_id = $2 AND is_pinned = true
`, profileID, folderID).Scan(&nextPin); err != nil {
		return err
	}
	order := nextPin
	if pinOrder != nil {
		order = *pinOrder
	}

	if folder.FolderType == "custom" {
		_, err = s.Pool.Exec(ctx, `
UPDATE folder_chats
SET is_pinned = true, pin_order = $4
WHERE profile_id = $1 AND folder_id = $2 AND chat_id = $3
`, profileID, folderID, chatID, order)
		return err
	}

	_, err = s.Pool.Exec(ctx, `
INSERT INTO folder_chats (profile_id, folder_id, chat_id, sort_order, is_pinned, pin_order)
VALUES ($1, $2, $3, 0, true, $4)
ON CONFLICT (profile_id, folder_id, chat_id) DO UPDATE
SET is_pinned = true, pin_order = EXCLUDED.pin_order
`, profileID, folderID, chatID, order)
	return err
}

// UnpinChatInFolder clears pin state for a chat in a folder.
func (s *DMStore) UnpinChatInFolder(ctx context.Context, profileID, folderID, chatID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	folder, err := s.GetFolder(ctx, profileID, folderID)
	if err != nil {
		return err
	}
	if folder.FolderType == "custom" {
		ct, err := s.Pool.Exec(ctx, `
UPDATE folder_chats
SET is_pinned = false, pin_order = NULL
WHERE profile_id = $1 AND folder_id = $2 AND chat_id = $3
`, profileID, folderID, chatID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrFolderChatNotMember
		}
		return nil
	}
	_, err = s.Pool.Exec(ctx, `
DELETE FROM folder_chats
WHERE profile_id = $1 AND folder_id = $2 AND chat_id = $3 AND is_pinned = true
`, profileID, folderID, chatID)
	return err
}

func systemFolderTypeFilter(cfg folderFilterConfig) (types []string, requireSpace bool) {
	switch cfg.SystemKey {
	case "dm":
		return []string{"dm"}, false
	case "group":
		return []string{"group"}, false
	case "channel":
		return []string{"channel"}, false
	case "spaces":
		return []string{"group", "channel"}, true
	default:
		return []string{"dm", "group", "channel"}, false
	}
}

func folderSystemKeyNeedsSpaceMerge(cfg folderFilterConfig) bool {
	key := strings.TrimSpace(cfg.SystemKey)
	return key == "" || key == "all" || key == "spaces"
}
