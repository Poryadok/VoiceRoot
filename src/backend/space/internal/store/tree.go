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
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	TreeKindTextChat  = "text_chat"
	TreeKindVoiceRoom = "voice_room"
	MaxTreeNodes      = 50
)

var (
	ErrTreeNodeLimit      = errors.New("space tree node limit reached")
	ErrInvalidTreeKind    = errors.New("invalid tree node kind")
	ErrTreeNodeNotFound   = errors.New("tree node not found")
	ErrCategoryNotFound   = errors.New("category not found")
	ErrVoiceRoomNotFound  = errors.New("voice room not found")
	ErrInvalidReorder     = errors.New("invalid tree reorder")
	ErrTreeNodeIDRequired = errors.New("tree node id required for update")
)

// CategoryRow is a row from categories.
type CategoryRow struct {
	ID        uuid.UUID
	SpaceID   uuid.UUID
	Name      string
	SortOrder int32
	CreatedAt time.Time
}

// VoiceRoomRow is a row from voice_rooms.
type VoiceRoomRow struct {
	ID        uuid.UUID
	SpaceID   uuid.UUID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TreeNodeRow is a row from space_tree_nodes.
type TreeNodeRow struct {
	ID          uuid.UUID
	SpaceID     uuid.UUID
	CategoryID  *uuid.UUID
	Kind        string
	ChatID      *uuid.UUID
	VoiceRoomID *uuid.UUID
	SortOrder   int32
	IsSystem    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SpaceTreeData aggregates categories, nodes, and voice rooms for a space.
type SpaceTreeData struct {
	Categories  []*CategoryRow
	Nodes       []*TreeNodeRow
	VoiceRooms  []*VoiceRoomRow
}

func (s *SpaceStore) nextTreeSortOrder(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID, categoryID *uuid.UUID) (int32, error) {
	var max sql.NullInt32
	if categoryID != nil {
		err := tx.QueryRow(ctx, `
SELECT COALESCE(MAX(sort_order), -1) FROM space_tree_nodes
WHERE space_id = $1 AND category_id = $2
`, spaceID, *categoryID).Scan(&max)
		if err != nil {
			return 0, err
		}
	} else {
		err := tx.QueryRow(ctx, `
SELECT COALESCE(MAX(sort_order), -1) FROM space_tree_nodes
WHERE space_id = $1 AND category_id IS NULL
`, spaceID).Scan(&max)
		if err != nil {
			return 0, err
		}
	}
	if !max.Valid {
		return 0, nil
	}
	return max.Int32 + 1, nil
}

func (s *SpaceStore) countTreeNodes(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT COUNT(*)::int FROM space_tree_nodes WHERE space_id = $1`, spaceID).Scan(&n)
	return n, err
}

// CreateCategory inserts a category for a space.
func (s *SpaceStore) CreateCategory(ctx context.Context, spaceID uuid.UUID, name string, sortOrder int32) (*CategoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("category name is required")
	}
	return scanCategoryRow(s.Pool.QueryRow(ctx, `
INSERT INTO categories (space_id, name, sort_order)
VALUES ($1, $2, $3)
RETURNING id, space_id, name, sort_order, created_at
`, spaceID, name, sortOrder))
}

// ListCategories returns categories for a space ordered by sort_order.
func (s *SpaceStore) ListCategories(ctx context.Context, spaceID uuid.UUID) ([]*CategoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, space_id, name, sort_order, created_at
FROM categories
WHERE space_id = $1
ORDER BY sort_order ASC, id ASC
`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CategoryRow
	for rows.Next() {
		row, err := scanCategoryRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// UpdateCategory updates category name and/or sort order.
func (s *SpaceStore) UpdateCategory(ctx context.Context, categoryID uuid.UUID, name *string, sortOrder *int32) (*CategoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if name == nil && sortOrder == nil {
		return scanCategoryRow(s.Pool.QueryRow(ctx, `
SELECT id, space_id, name, sort_order, created_at FROM categories WHERE id = $1
`, categoryID))
	}
	sets := make([]string, 0, 2)
	args := make([]any, 0, 3)
	argN := 1
	if name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argN))
		args = append(args, strings.TrimSpace(*name))
		argN++
	}
	if sortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order = $%d", argN))
		args = append(args, *sortOrder)
		argN++
	}
	args = append(args, categoryID)
	q := fmt.Sprintf(`
UPDATE categories SET %s WHERE id = $%d
RETURNING id, space_id, name, sort_order, created_at
`, strings.Join(sets, ", "), argN)
	row, err := scanCategoryRow(s.Pool.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCategoryNotFound
	}
	return row, err
}

// DeleteCategory removes a category; tree nodes get category_id = NULL.
func (s *SpaceStore) DeleteCategory(ctx context.Context, categoryID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, categoryID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// CreateVoiceRoom creates a voice room and its tree node in one transaction.
func (s *SpaceStore) CreateVoiceRoom(ctx context.Context, spaceID uuid.UUID, name string, categoryID *uuid.UUID) (*VoiceRoomRow, *TreeNodeRow, error) {
	if s == nil || s.Pool == nil {
		return nil, nil, errors.New("space store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil, errors.New("voice room name is required")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	n, err := s.countTreeNodes(ctx, tx, spaceID)
	if err != nil {
		return nil, nil, err
	}
	if n >= MaxTreeNodes {
		return nil, nil, ErrTreeNodeLimit
	}

	room, err := scanVoiceRoomRow(tx.QueryRow(ctx, `
INSERT INTO voice_rooms (space_id, name)
VALUES ($1, $2)
RETURNING id, space_id, name, created_at, updated_at
`, spaceID, name))
	if err != nil {
		return nil, nil, err
	}

	sortOrder, err := s.nextTreeSortOrder(ctx, tx, spaceID, categoryID)
	if err != nil {
		return nil, nil, err
	}

	node, err := scanTreeNodeRow(tx.QueryRow(ctx, `
INSERT INTO space_tree_nodes (space_id, category_id, kind, voice_room_id, sort_order)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, space_id, category_id, kind, chat_id, voice_room_id, sort_order, is_system, created_at, updated_at
`, spaceID, categoryID, TreeKindVoiceRoom, room.ID, sortOrder))
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return room, node, nil
}

// UpdateVoiceRoom renames a voice room.
func (s *SpaceStore) UpdateVoiceRoom(ctx context.Context, voiceRoomID uuid.UUID, name string) (*VoiceRoomRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("voice room name is required")
	}
	row, err := scanVoiceRoomRow(s.Pool.QueryRow(ctx, `
UPDATE voice_rooms SET name = $1, updated_at = now() WHERE id = $2
RETURNING id, space_id, name, created_at, updated_at
`, name, voiceRoomID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVoiceRoomNotFound
	}
	return row, err
}

// DeleteVoiceRoom deletes a voice room; tree node cascades via FK.
func (s *SpaceStore) DeleteVoiceRoom(ctx context.Context, voiceRoomID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM voice_rooms WHERE id = $1`, voiceRoomID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVoiceRoomNotFound
	}
	return nil
}

// UpsertTreeNodeInput holds fields for creating or updating a tree node.
type UpsertTreeNodeInput struct {
	SpaceID     uuid.UUID
	NodeID      *uuid.UUID
	CategoryID  *uuid.UUID
	Kind        string
	ChatID      *uuid.UUID
	VoiceRoomID *uuid.UUID
	SortOrder   *int32
	IsSystem    *bool
}

// UpsertTreeNode creates or updates a tree node.
func (s *SpaceStore) UpsertTreeNode(ctx context.Context, in UpsertTreeNodeInput) (*TreeNodeRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	kind := strings.TrimSpace(in.Kind)
	if kind != TreeKindTextChat && kind != TreeKindVoiceRoom {
		return nil, ErrInvalidTreeKind
	}
	if kind == TreeKindTextChat && (in.ChatID == nil || *in.ChatID == uuid.Nil) {
		return nil, errors.New("chat_id is required for text_chat")
	}
	if kind == TreeKindVoiceRoom && (in.VoiceRoomID == nil || *in.VoiceRoomID == uuid.Nil) {
		return nil, errors.New("voice_room_id is required for voice_room")
	}

	if in.NodeID != nil && *in.NodeID != uuid.Nil {
		return s.updateTreeNode(ctx, in)
	}
	return s.insertTreeNode(ctx, in)
}

func (s *SpaceStore) insertTreeNode(ctx context.Context, in UpsertTreeNodeInput) (*TreeNodeRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	n, err := s.countTreeNodes(ctx, tx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	if n >= MaxTreeNodes {
		return nil, ErrTreeNodeLimit
	}

	sortOrder := int32(0)
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	} else {
		sortOrder, err = s.nextTreeSortOrder(ctx, tx, in.SpaceID, in.CategoryID)
		if err != nil {
			return nil, err
		}
	}
	isSystem := false
	if in.IsSystem != nil {
		isSystem = *in.IsSystem
	}

	var node *TreeNodeRow
	if in.Kind == TreeKindTextChat {
		node, err = scanTreeNodeRow(tx.QueryRow(ctx, `
INSERT INTO space_tree_nodes (space_id, category_id, kind, chat_id, sort_order, is_system)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, space_id, category_id, kind, chat_id, voice_room_id, sort_order, is_system, created_at, updated_at
`, in.SpaceID, in.CategoryID, TreeKindTextChat, *in.ChatID, sortOrder, isSystem))
	} else {
		node, err = scanTreeNodeRow(tx.QueryRow(ctx, `
INSERT INTO space_tree_nodes (space_id, category_id, kind, voice_room_id, sort_order, is_system)
VALUES ($1, $2, $3, $4, $5, false)
RETURNING id, space_id, category_id, kind, chat_id, voice_room_id, sort_order, is_system, created_at, updated_at
`, in.SpaceID, in.CategoryID, TreeKindVoiceRoom, *in.VoiceRoomID, sortOrder))
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return node, nil
}

func (s *SpaceStore) updateTreeNode(ctx context.Context, in UpsertTreeNodeInput) (*TreeNodeRow, error) {
	sets := []string{"updated_at = now()"}
	args := make([]any, 0, 8)
	argN := 1

	if in.CategoryID != nil {
		sets = append(sets, fmt.Sprintf("category_id = $%d", argN))
		args = append(args, *in.CategoryID)
		argN++
	}
	if in.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order = $%d", argN))
		args = append(args, *in.SortOrder)
		argN++
	}
	if in.IsSystem != nil && in.Kind == TreeKindTextChat {
		sets = append(sets, fmt.Sprintf("is_system = $%d", argN))
		args = append(args, *in.IsSystem)
		argN++
	}

	args = append(args, *in.NodeID, in.SpaceID)
	q := fmt.Sprintf(`
UPDATE space_tree_nodes SET %s WHERE id = $%d AND space_id = $%d
RETURNING id, space_id, category_id, kind, chat_id, voice_room_id, sort_order, is_system, created_at, updated_at
`, strings.Join(sets, ", "), argN, argN+1)

	row, err := scanTreeNodeRow(s.Pool.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTreeNodeNotFound
	}
	return row, err
}

// RemoveTreeNode deletes a tree node by id.
func (s *SpaceStore) RemoveTreeNode(ctx context.Context, spaceID, nodeID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	tag, err := s.Pool.Exec(ctx, `
DELETE FROM space_tree_nodes WHERE id = $1 AND space_id = $2
`, nodeID, spaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTreeNodeNotFound
	}
	return nil
}

// ListTreeNodes returns tree nodes for a space ordered by sort_order.
func (s *SpaceStore) ListTreeNodes(ctx context.Context, spaceID uuid.UUID) ([]*TreeNodeRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, space_id, category_id, kind, chat_id, voice_room_id, sort_order, is_system, created_at, updated_at
FROM space_tree_nodes
WHERE space_id = $1
ORDER BY sort_order ASC, id ASC
`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TreeNodeRow
	for rows.Next() {
		row, err := scanTreeNodeRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListVoiceRooms returns voice rooms for a space.
func (s *SpaceStore) ListVoiceRooms(ctx context.Context, spaceID uuid.UUID) ([]*VoiceRoomRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, space_id, name, created_at, updated_at
FROM voice_rooms
WHERE space_id = $1
ORDER BY created_at ASC
`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*VoiceRoomRow
	for rows.Next() {
		row, err := scanVoiceRoomRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListSpaceTree loads categories, nodes, and voice rooms for a space.
func (s *SpaceStore) ListSpaceTree(ctx context.Context, spaceID uuid.UUID) (*SpaceTreeData, error) {
	categories, err := s.ListCategories(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	nodes, err := s.ListTreeNodes(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	rooms, err := s.ListVoiceRooms(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	return &SpaceTreeData{
		Categories: categories,
		Nodes:      nodes,
		VoiceRooms: rooms,
	}, nil
}

// ReorderSpaceTree assigns sort_order 0..n-1 from ordered node ids.
func (s *SpaceStore) ReorderSpaceTree(ctx context.Context, spaceID uuid.UUID, orderedNodeIDs []uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	if len(orderedNodeIDs) == 0 {
		return nil
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existing int
	err = tx.QueryRow(ctx, `
SELECT COUNT(*)::int FROM space_tree_nodes WHERE space_id = $1 AND id = ANY($2)
`, spaceID, orderedNodeIDs).Scan(&existing)
	if err != nil {
		return err
	}
	if existing != len(orderedNodeIDs) {
		return ErrInvalidReorder
	}

	for i, id := range orderedNodeIDs {
		tag, err := tx.Exec(ctx, `
UPDATE space_tree_nodes SET sort_order = $1, updated_at = now()
WHERE id = $2 AND space_id = $3
`, int32(i), id, spaceID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrInvalidReorder
		}
	}
	return tx.Commit(ctx)
}

func scanCategoryRow(row pgx.Row) (*CategoryRow, error) {
	var id, spaceID uuid.UUID
	var name string
	var sortOrder int32
	var createdAt time.Time
	err := row.Scan(&id, &spaceID, &name, &sortOrder, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &CategoryRow{
		ID:        id,
		SpaceID:   spaceID,
		Name:      name,
		SortOrder: sortOrder,
		CreatedAt: createdAt.UTC(),
	}, nil
}

func scanVoiceRoomRow(row pgx.Row) (*VoiceRoomRow, error) {
	var id, spaceID uuid.UUID
	var name string
	var createdAt, updatedAt time.Time
	err := row.Scan(&id, &spaceID, &name, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVoiceRoomNotFound
	}
	if err != nil {
		return nil, err
	}
	return &VoiceRoomRow{
		ID:        id,
		SpaceID:   spaceID,
		Name:      name,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: updatedAt.UTC(),
	}, nil
}

func scanTreeNodeRow(row pgx.Row) (*TreeNodeRow, error) {
	var id, spaceID uuid.UUID
	var kind string
	var categoryID, chatID, voiceRoomID pgtype.UUID
	var sortOrder int32
	var isSystem bool
	var createdAt, updatedAt time.Time
	err := row.Scan(&id, &spaceID, &categoryID, &kind, &chatID, &voiceRoomID, &sortOrder, &isSystem, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTreeNodeNotFound
	}
	if err != nil {
		return nil, err
	}
	out := &TreeNodeRow{
		ID:        id,
		SpaceID:   spaceID,
		Kind:      kind,
		SortOrder: sortOrder,
		IsSystem:  isSystem,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: updatedAt.UTC(),
	}
	if categoryID.Valid {
		cid := uuid.UUID(categoryID.Bytes)
		out.CategoryID = &cid
	}
	if chatID.Valid {
		cid := uuid.UUID(chatID.Bytes)
		out.ChatID = &cid
	}
	if voiceRoomID.Valid {
		vid := uuid.UUID(voiceRoomID.Bytes)
		out.VoiceRoomID = &vid
	}
	return out, nil
}
