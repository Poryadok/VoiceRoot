package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrNotImplemented = errors.New("story store: not implemented")
	ErrForbidden      = errors.New("forbidden")
)

// StoryStore persists stories, views, reactions, and highlights.
type StoryStore struct {
	Pool *pgxpool.Pool
}

// StoryRow is a persisted story record.
type StoryRow struct {
	ID                uuid.UUID
	AuthorProfileID   uuid.UUID
	Type              string
	MediaFileID       *uuid.UUID
	TextContent       *string
	TextStyleJSON     *string
	GameTag           *string
	IsLookingForParty bool
	LFPCriteriaJSON   *string
	MentionProfileIDs string
	ViewCount         int
	Visibility        string
	ExpiresAt         time.Time
	ArchivedUntil     time.Time
	CreatedAt         time.Time
	DeletedAt         *time.Time
	ExpiredAt         *time.Time
}

// HighlightRow is a profile highlight collection.
type HighlightRow struct {
	ID          uuid.UUID
	ProfileID   uuid.UUID
	Name        string
	CoverFileID *uuid.UUID
	SortOrder   int
	Visibility  string
	StoryIDs    []uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// StoryReactionRow is a persisted emoji reaction.
type StoryReactionRow struct {
	ReactorProfileID uuid.UUID
	Emoji            string
}

// ArchivedStoryMedia is a story row eligible for archive purge with optional media.
type ArchivedStoryMedia struct {
	StoryID     uuid.UUID
	MediaFileID *uuid.UUID
}

// PaginatedStories is the result of cursor-based active story listing.
type PaginatedStories struct {
	Rows       []StoryRow
	NextCursor string
	HasMore    bool
}

type CreateStoryInput struct {
	AuthorProfileID   uuid.UUID
	Type              string
	MediaFileID       *uuid.UUID
	TextContent       *string
	TextStyleJSON     *string
	GameTag           *string
	IsLookingForParty bool
	LFPCriteriaJSON   *string
	MentionProfileIDs string
	Visibility        string
}

func storyTTL() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("STORY_TTL_DEV")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 24 * time.Hour
}

func archiveRetention() time.Duration {
	return 30 * 24 * time.Hour
}

// CreateStory inserts a new story with configurable TTL (default 24h).
func (s *StoryStore) CreateStory(ctx context.Context, in CreateStoryInput) (*StoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	ttl := storyTTL()
	now := time.Now().UTC()
	id := uuid.New()
	mentions := strings.TrimSpace(in.MentionProfileIDs)
	if mentions == "" {
		mentions = "[]"
	}
	expiresAt := now.Add(ttl)
	archivedUntil := expiresAt.Add(archiveRetention())

	_, err := s.Pool.Exec(ctx, `
INSERT INTO stories (
  id, author_profile_id, type, media_file_id, text_content, text_style,
  game_tag, is_looking_for_party, lfp_criteria, mention_profile_ids,
  visibility, expires_at, archived_until, created_at
) VALUES (
  $1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9::jsonb, $10::jsonb,
  $11, $12, $13, $14
)`,
		id, in.AuthorProfileID, in.Type, in.MediaFileID, in.TextContent, nullableJSON(in.TextStyleJSON),
		in.GameTag, in.IsLookingForParty, nullableJSON(in.LFPCriteriaJSON), mentions,
		in.Visibility, expiresAt, archivedUntil, now,
	)
	if err != nil {
		return nil, err
	}
	return s.GetStory(ctx, id)
}

func nullableJSON(s *string) any {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return *s
}

// GetStory loads a story by id (not found when soft-deleted).
func (s *StoryStore) GetStory(ctx context.Context, storyID uuid.UUID) (*StoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	row := s.Pool.QueryRow(ctx, `
SELECT id, author_profile_id, type, media_file_id, text_content,
       text_style::text, game_tag, is_looking_for_party, lfp_criteria::text,
       mention_profile_ids::text, view_count, visibility, expires_at, archived_until,
       created_at, deleted_at, expired_at
FROM stories
WHERE id = $1 AND deleted_at IS NULL`, storyID)
	out, err := scanStory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return out, err
}

func scanStory(row pgx.Row) (*StoryRow, error) {
	var out StoryRow
	var mediaID *uuid.UUID
	var textStyle, lfp, mentions *string
	err := row.Scan(
		&out.ID, &out.AuthorProfileID, &out.Type, &mediaID, &out.TextContent,
		&textStyle, &out.GameTag, &out.IsLookingForParty, &lfp,
		&mentions, &out.ViewCount, &out.Visibility, &out.ExpiresAt, &out.ArchivedUntil,
		&out.CreatedAt, &out.DeletedAt, &out.ExpiredAt,
	)
	if err != nil {
		return nil, err
	}
	out.MediaFileID = mediaID
	out.TextStyleJSON = textStyle
	out.LFPCriteriaJSON = lfp
	if mentions != nil {
		out.MentionProfileIDs = *mentions
	} else {
		out.MentionProfileIDs = "[]"
	}
	return &out, nil
}

// DeleteStory soft-deletes a story owned by authorProfileID.
func (s *StoryStore) DeleteStory(ctx context.Context, storyID, authorProfileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	tag, err := s.Pool.Exec(ctx, `
UPDATE stories SET deleted_at = now()
WHERE id = $1 AND author_profile_id = $2 AND deleted_at IS NULL`, storyID, authorProfileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkViewed records a view; anonymous views omit viewer from author list.
func (s *StoryStore) MarkViewed(ctx context.Context, storyID, viewerProfileID uuid.UUID, anonymous bool) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists bool
	err = tx.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM story_views WHERE story_id = $1 AND viewer_profile_id = $2
)`, storyID, viewerProfileID).Scan(&exists)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO story_views (story_id, viewer_profile_id, is_anonymous, viewed_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (story_id, viewer_profile_id) DO UPDATE SET
  is_anonymous = EXCLUDED.is_anonymous,
  viewed_at = now()`, storyID, viewerProfileID, anonymous)
	if err != nil {
		return err
	}
	if !exists {
		_, err = tx.Exec(ctx, `
UPDATE stories SET view_count = view_count + 1
WHERE id = $1 AND deleted_at IS NULL`, storyID)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListViewers returns non-anonymous viewer profile IDs for an active story.
func (s *StoryStore) ListViewers(ctx context.Context, storyID uuid.UUID) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	rows, err := s.Pool.Query(ctx, `
SELECT sv.viewer_profile_id
FROM story_views sv
JOIN stories s ON s.id = sv.story_id
WHERE sv.story_id = $1 AND sv.is_anonymous = false
  AND s.deleted_at IS NULL AND s.expired_at IS NULL`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ReactToStory upserts an emoji reaction from reactorProfileID.
func (s *StoryStore) ReactToStory(ctx context.Context, storyID, reactorProfileID uuid.UUID, emoji string) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO story_reactions (story_id, reactor_profile_id, emoji, created_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (story_id, reactor_profile_id) DO UPDATE SET emoji = EXCLUDED.emoji, created_at = now()`,
		storyID, reactorProfileID, emoji)
	return err
}

// ListActiveStoriesPaginated returns active stories using created_at|id cursor pagination.
func (s *StoryStore) ListActiveStoriesPaginated(ctx context.Context, limit int, cursor string) (*PaginatedStories, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	if limit <= 0 {
		limit = 50
	}
	fetch := limit + 1
	cursorTime, cursorID, err := decodeStoryCursor(cursor)
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	if cursorTime == nil {
		rows, err = s.Pool.Query(ctx, `
SELECT id, author_profile_id, type, media_file_id, text_content,
       text_style::text, game_tag, is_looking_for_party, lfp_criteria::text,
       mention_profile_ids::text, view_count, visibility, expires_at, archived_until,
       created_at, deleted_at, expired_at
FROM stories
WHERE deleted_at IS NULL AND expired_at IS NULL AND expires_at > now()
ORDER BY created_at DESC, id DESC
LIMIT $1`, fetch)
	} else {
		rows, err = s.Pool.Query(ctx, `
SELECT id, author_profile_id, type, media_file_id, text_content,
       text_style::text, game_tag, is_looking_for_party, lfp_criteria::text,
       mention_profile_ids::text, view_count, visibility, expires_at, archived_until,
       created_at, deleted_at, expired_at
FROM stories
WHERE deleted_at IS NULL AND expired_at IS NULL AND expires_at > now()
  AND (created_at, id) < ($1, $2)
ORDER BY created_at DESC, id DESC
LIMIT $3`, *cursorTime, *cursorID, fetch)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	all, err := scanStoryRows(rows)
	if err != nil {
		return nil, err
	}
	out := &PaginatedStories{}
	if len(all) > limit {
		out.HasMore = true
		all = all[:limit]
	}
	out.Rows = all
	if out.HasMore && len(all) > 0 {
		last := all[len(all)-1]
		out.NextCursor = encodeStoryCursor(last.CreatedAt, last.ID)
	}
	return out, nil
}

func encodeStoryCursor(t time.Time, id uuid.UUID) string {
	return fmt.Sprintf("%s|%s", t.UTC().Format(time.RFC3339Nano), id.String())
}

func decodeStoryCursor(cursor string) (*time.Time, *uuid.UUID, error) {
	if strings.TrimSpace(cursor) == "" {
		return nil, nil, nil
	}
	parts := strings.SplitN(cursor, "|", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor time")
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor id")
	}
	return &t, &id, nil
}

// ListStoryReactions returns emoji reactions for a story.
func (s *StoryStore) ListStoryReactions(ctx context.Context, storyID uuid.UUID) ([]StoryReactionRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	rows, err := s.Pool.Query(ctx, `
SELECT reactor_profile_id, emoji
FROM story_reactions
WHERE story_id = $1
ORDER BY created_at`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoryReactionRow
	for rows.Next() {
		var row StoryReactionRow
		if err := rows.Scan(&row.ReactorProfileID, &row.Emoji); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListArchivedStoriesForPurge returns expired stories past archived_until with media ids.
func (s *StoryStore) ListArchivedStoriesForPurge(ctx context.Context, now time.Time) ([]ArchivedStoryMedia, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, media_file_id
FROM stories
WHERE archived_until <= $1 AND expired_at IS NOT NULL`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArchivedStoryMedia
	for rows.Next() {
		var row ArchivedStoryMedia
		if err := rows.Scan(&row.StoryID, &row.MediaFileID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListActiveStories returns active (non-expired, non-deleted) stories for feed.
func (s *StoryStore) ListActiveStories(ctx context.Context, limit int) ([]StoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, author_profile_id, type, media_file_id, text_content,
       text_style::text, game_tag, is_looking_for_party, lfp_criteria::text,
       mention_profile_ids::text, view_count, visibility, expires_at, archived_until,
       created_at, deleted_at, expired_at
FROM stories
WHERE deleted_at IS NULL AND expired_at IS NULL AND expires_at > now()
ORDER BY created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStoryRows(rows)
}

// ListActiveStoriesByAuthor returns active stories for one profile.
func (s *StoryStore) ListActiveStoriesByAuthor(ctx context.Context, authorID uuid.UUID) ([]StoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, author_profile_id, type, media_file_id, text_content,
       text_style::text, game_tag, is_looking_for_party, lfp_criteria::text,
       mention_profile_ids::text, view_count, visibility, expires_at, archived_until,
       created_at, deleted_at, expired_at
FROM stories
WHERE author_profile_id = $1 AND deleted_at IS NULL AND expired_at IS NULL AND expires_at > now()
ORDER BY created_at DESC`, authorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStoryRows(rows)
}

func scanStoryRows(rows pgx.Rows) ([]StoryRow, error) {
	var out []StoryRow
	for rows.Next() {
		var row StoryRow
		var mediaID *uuid.UUID
		var textStyle, lfp, mentions *string
		if err := rows.Scan(
			&row.ID, &row.AuthorProfileID, &row.Type, &mediaID, &row.TextContent,
			&textStyle, &row.GameTag, &row.IsLookingForParty, &lfp,
			&mentions, &row.ViewCount, &row.Visibility, &row.ExpiresAt, &row.ArchivedUntil,
			&row.CreatedAt, &row.DeletedAt, &row.ExpiredAt,
		); err != nil {
			return nil, err
		}
		row.MediaFileID = mediaID
		row.TextStyleJSON = textStyle
		row.LFPCriteriaJSON = lfp
		if mentions != nil {
			row.MentionProfileIDs = *mentions
		} else {
			row.MentionProfileIDs = "[]"
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListArchive returns archived stories for profileID.
func (s *StoryStore) ListArchive(ctx context.Context, profileID uuid.UUID) ([]StoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, author_profile_id, type, media_file_id, text_content,
       text_style::text, game_tag, is_looking_for_party, lfp_criteria::text,
       mention_profile_ids::text, view_count, visibility, expires_at, archived_until,
       created_at, deleted_at, expired_at
FROM stories
WHERE author_profile_id = $1 AND deleted_at IS NULL AND expired_at IS NOT NULL
  AND archived_until > now()
ORDER BY created_at DESC`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStoryRows(rows)
}

// MarkExpiredStories transitions active stories past expires_at to expired state.
func (s *StoryStore) MarkExpiredStories(ctx context.Context, now time.Time) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, ErrNotImplemented
	}
	tag, err := s.Pool.Exec(ctx, `
UPDATE stories SET expired_at = $1
WHERE deleted_at IS NULL AND expired_at IS NULL AND expires_at <= $1`, now.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// CreateHighlight creates a named highlight for profileID.
func (s *StoryStore) CreateHighlight(ctx context.Context, profileID uuid.UUID, name, visibility string) (*HighlightRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	if strings.TrimSpace(visibility) == "" {
		visibility = "everyone"
	}
	id := uuid.New()
	now := time.Now().UTC()
	_, err := s.Pool.Exec(ctx, `
INSERT INTO highlights (id, profile_id, name, sort_order, visibility, created_at, updated_at)
VALUES ($1, $2, $3, 0, $4, $5, $5)`, id, profileID, name, visibility, now)
	if err != nil {
		return nil, err
	}
	return s.getHighlight(ctx, id, profileID)
}

func (s *StoryStore) getHighlight(ctx context.Context, highlightID, profileID uuid.UUID) (*HighlightRow, error) {
	row := s.Pool.QueryRow(ctx, `
SELECT id, profile_id, name, cover_file_id, sort_order, visibility, created_at, updated_at
FROM highlights WHERE id = $1 AND profile_id = $2`, highlightID, profileID)
	var out HighlightRow
	var cover *uuid.UUID
	err := row.Scan(&out.ID, &out.ProfileID, &out.Name, &cover, &out.SortOrder, &out.Visibility, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out.CoverFileID = cover
	storyIDs, err := s.highlightStoryIDs(ctx, highlightID)
	if err != nil {
		return nil, err
	}
	out.StoryIDs = storyIDs
	return &out, nil
}

func (s *StoryStore) highlightStoryIDs(ctx context.Context, highlightID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT story_id FROM highlight_stories WHERE highlight_id = $1 ORDER BY sort_order, added_at`, highlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpdateHighlight renames a highlight owned by profileID.
func (s *StoryStore) UpdateHighlight(ctx context.Context, highlightID, profileID uuid.UUID, name, visibility string) (*HighlightRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	if strings.TrimSpace(name) != "" && strings.TrimSpace(visibility) != "" {
		tag, err := s.Pool.Exec(ctx, `
UPDATE highlights SET name = $3, visibility = $4, updated_at = now()
WHERE id = $1 AND profile_id = $2`, highlightID, profileID, name, visibility)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, ErrNotFound
		}
		return s.getHighlight(ctx, highlightID, profileID)
	}
	if strings.TrimSpace(name) != "" {
		tag, err := s.Pool.Exec(ctx, `
UPDATE highlights SET name = $3, updated_at = now()
WHERE id = $1 AND profile_id = $2`, highlightID, profileID, name)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, ErrNotFound
		}
		return s.getHighlight(ctx, highlightID, profileID)
	}
	if strings.TrimSpace(visibility) != "" {
		tag, err := s.Pool.Exec(ctx, `
UPDATE highlights SET visibility = $3, updated_at = now()
WHERE id = $1 AND profile_id = $2`, highlightID, profileID, visibility)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, ErrNotFound
		}
		return s.getHighlight(ctx, highlightID, profileID)
	}
	return nil, ErrNotFound
}

// DeleteHighlight removes a highlight owned by profileID.
func (s *StoryStore) DeleteHighlight(ctx context.Context, highlightID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM highlights WHERE id = $1 AND profile_id = $2`, highlightID, profileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddToHighlight links a story to a highlight.
func (s *StoryStore) AddToHighlight(ctx context.Context, highlightID, profileID, storyID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	var owner uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT profile_id FROM highlights WHERE id = $1`, highlightID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != profileID {
		return ErrForbidden
	}
	var storyAuthor uuid.UUID
	err = s.Pool.QueryRow(ctx, `SELECT author_profile_id FROM stories WHERE id = $1 AND deleted_at IS NULL`, storyID).Scan(&storyAuthor)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if storyAuthor != profileID {
		return ErrForbidden
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO highlight_stories (highlight_id, story_id, sort_order, added_at)
VALUES ($1, $2, 0, now())
ON CONFLICT (highlight_id, story_id) DO NOTHING`, highlightID, storyID)
	return err
}

// RemoveFromHighlight unlinks a story from a highlight.
func (s *StoryStore) RemoveFromHighlight(ctx context.Context, highlightID, profileID, storyID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	tag, err := s.Pool.Exec(ctx, `
DELETE FROM highlight_stories hs
USING highlights h
WHERE hs.highlight_id = h.id AND h.id = $1 AND h.profile_id = $2 AND hs.story_id = $3`,
		highlightID, profileID, storyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetHighlights lists highlights for profileID.
func (s *StoryStore) GetHighlights(ctx context.Context, profileID uuid.UUID) ([]HighlightRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, profile_id, name, cover_file_id, sort_order, visibility, created_at, updated_at
FROM highlights WHERE profile_id = $1 ORDER BY sort_order, created_at`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HighlightRow
	for rows.Next() {
		var row HighlightRow
		var cover *uuid.UUID
		if err := rows.Scan(&row.ID, &row.ProfileID, &row.Name, &cover, &row.SortOrder, &row.Visibility, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		row.CoverFileID = cover
		storyIDs, err := s.highlightStoryIDs(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		row.StoryIDs = storyIDs
		out = append(out, row)
	}
	return out, rows.Err()
}

// PurgeArchivedStories deletes stories past archived_until.
func (s *StoryStore) PurgeArchivedStories(ctx context.Context, now time.Time) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, ErrNotImplemented
	}
	tag, err := s.Pool.Exec(ctx, `
DELETE FROM stories WHERE archived_until <= $1 AND expired_at IS NOT NULL`, now.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
