package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FolderRow is one chat folder for a profile.
type FolderRow struct {
	ID               uuid.UUID
	ProfileID        uuid.UUID
	Name             string
	FolderType       string
	FilterConfigJSON string
	SortOrder        int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type systemFolderDef struct {
	name           string
	sortOrder      int32
	filterConfig   string
}

var systemFolderDefs = []systemFolderDef{
	{name: "All", sortOrder: 0, filterConfig: `{"system_key":"all"}`},
	{name: "DM", sortOrder: 1, filterConfig: `{"system_key":"dm","chat_type":"dm"}`},
	{name: "Groups", sortOrder: 2, filterConfig: `{"system_key":"group","chat_type":"group"}`},
	{name: "Channels", sortOrder: 3, filterConfig: `{"system_key":"channel","chat_type":"channel"}`},
	{name: "Spaces", sortOrder: 4, filterConfig: `{"system_key":"spaces","has_space_id":true}`},
}

// EnsureSystemFolders seeds the five system folders when none exist for the profile.
func (s *DMStore) EnsureSystemFolders(ctx context.Context, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	var count int
	if err := s.Pool.QueryRow(ctx, `
SELECT COUNT(*) FROM folders WHERE profile_id = $1 AND folder_type = 'system'
`, profileID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, def := range systemFolderDefs {
		_, err := s.Pool.Exec(ctx, `
INSERT INTO folders (profile_id, name, folder_type, filter_config_json, sort_order)
VALUES ($1, $2, 'system', $3::jsonb, $4)
`, profileID, def.name, def.filterConfig, def.sortOrder)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListFolders returns folders for a profile, lazily seeding system folders first.
func (s *DMStore) ListFolders(ctx context.Context, profileID uuid.UUID) ([]FolderRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if err := s.EnsureSystemFolders(ctx, profileID); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, profile_id, name, folder_type, filter_config_json::text, sort_order, created_at, updated_at
FROM folders
WHERE profile_id = $1
ORDER BY sort_order ASC, created_at ASC
`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FolderRow
	for rows.Next() {
		var row FolderRow
		if err := rows.Scan(&row.ID, &row.ProfileID, &row.Name, &row.FolderType, &row.FilterConfigJSON, &row.SortOrder, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// CreateFolder creates a custom folder for the caller profile.
func (s *DMStore) CreateFolder(ctx context.Context, profileID uuid.UUID, name, filterConfigJSON string) (*FolderRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("folder name is required")
	}
	filterJSON := filterConfigJSON
	if filterJSON == "" {
		filterJSON = "{}"
	}
	if err := s.EnsureSystemFolders(ctx, profileID); err != nil {
		return nil, err
	}
	var nextOrder int32
	if err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(MAX(sort_order), -1) + 1 FROM folders WHERE profile_id = $1
`, profileID).Scan(&nextOrder); err != nil {
		return nil, err
	}
	var row FolderRow
	err := s.Pool.QueryRow(ctx, `
INSERT INTO folders (profile_id, name, folder_type, filter_config_json, sort_order)
VALUES ($1, $2, 'custom', $3::jsonb, $4)
RETURNING id, profile_id, name, folder_type, filter_config_json::text, sort_order, created_at, updated_at
`, profileID, name, filterJSON, nextOrder).Scan(
		&row.ID, &row.ProfileID, &row.Name, &row.FolderType, &row.FilterConfigJSON, &row.SortOrder, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
