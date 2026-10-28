package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateStoryWithMedia persists recovery ownership before calling File. The
// intent lock serializes publication with recovery, including uncertain commits.
func (s *StoryStore) CreateStoryWithMedia(ctx context.Context, in CreateStoryInput, acquire func(context.Context, uuid.UUID) error) (*StoryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrNotImplemented
	}
	if in.MediaFileID == nil || acquire == nil {
		return nil, errors.New("media acquisition is required")
	}
	id := uuid.New()
	intentTx, err := s.beginInventoryTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = intentTx.Rollback(ctx) }()
	installed, err := inventoryInstalled(ctx, intentTx)
	if err != nil {
		return nil, err
	}
	if installed {
		_, err = intentTx.Exec(ctx, `INSERT INTO story_media_reference_intents(story_id,media_file_id,author_profile_id,media_type,available_at) VALUES($1,$2,$3,$4,now()+interval '1 minute')`, id, *in.MediaFileID, in.AuthorProfileID, in.Type)
	} else {
		_, err = intentTx.Exec(ctx, `INSERT INTO story_media_reference_intents(story_id,media_file_id,available_at) VALUES($1,$2,now()+interval '1 minute')`, id, *in.MediaFileID)
	}
	if err != nil {
		return nil, err
	}
	if err = recordInventory(ctx, intentTx, []uuid.UUID{id}); err != nil {
		return nil, err
	}
	if err = intentTx.Commit(ctx); err != nil {
		return nil, err
	}
	tx, err := s.beginInventoryTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var locked uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT story_id FROM story_media_reference_intents WHERE story_id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
		return nil, err
	}
	if err = acquire(ctx, id); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	expires := now.Add(storyTTL())
	mentions := strings.TrimSpace(in.MentionProfileIDs)
	if mentions == "" {
		mentions = "[]"
	}
	_, err = tx.Exec(ctx, `INSERT INTO stories(id,author_profile_id,type,media_file_id,text_content,text_style,game_tag,is_looking_for_party,lfp_criteria,mention_profile_ids,visibility,visibility_audience,expires_at,archived_until,created_at) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9::jsonb,$10::jsonb,$11,$12::jsonb,$13,$14,$15)`, id, in.AuthorProfileID, in.Type, in.MediaFileID, in.TextContent, nullableJSON(in.TextStyleJSON), in.GameTag, in.IsLookingForParty, nullableJSON(in.LFPCriteriaJSON), mentions, in.Visibility, nullableJSON(in.VisibilityAudienceJSON), expires, expires.Add(archiveRetention()), now)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM story_media_reference_intents WHERE story_id=$1`, id); err != nil {
		return nil, err
	}
	if err = recordInventory(ctx, tx, []uuid.UUID{id}); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetStory(ctx, id)
}

// RecoverAbandonedMedia releases one pending creation after obtaining its lock.
// File records a retired exact key even when acquire never committed, so a late
// acquire cannot resurrect it. The intent survives any ambiguous release.
func (s *StoryStore) RecoverAbandonedMedia(ctx context.Context, release func(context.Context, uuid.UUID, uuid.UUID) error) (bool, error) {
	if s == nil || s.Pool == nil || release == nil {
		return false, ErrNotImplemented
	}
	tx, err := s.beginInventoryTx(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var story, file uuid.UUID
	err = tx.QueryRow(ctx, `SELECT story_id,media_file_id FROM story_media_reference_intents WHERE available_at<=now() ORDER BY available_at,story_id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&story, &file)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var visible bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stories WHERE id=$1)`, story).Scan(&visible); err != nil {
		return false, err
	}
	if visible {
		return false, errors.New("media intent unexpectedly has a published story")
	}
	if err = release(ctx, story, file); err != nil {
		if _, updateErr := tx.Exec(ctx, `UPDATE story_media_reference_intents SET available_at=now()+interval '1 minute' WHERE story_id=$1`, story); updateErr != nil {
			return false, updateErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return false, commitErr
		}
		return false, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM story_media_reference_intents WHERE story_id=$1`, story); err != nil {
		return false, err
	}
	installed, err := inventoryInstalled(ctx, tx)
	if err != nil {
		return false, err
	}
	if installed {
		if _, err = tx.Exec(ctx, `INSERT INTO story_reference_inventory_retirements(story_id,media_file_id,operation_id) VALUES($1,$2,$1) ON CONFLICT DO NOTHING`, story, file); err != nil {
			return false, err
		}
	}
	if err = recordInventory(ctx, tx, []uuid.UUID{story}); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *StoryStore) PruneDeletionReceipts(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM story_deletion_receipts WHERE story_id IN (SELECT story_id FROM story_deletion_receipts WHERE completed_at<=now()-interval '30 days' ORDER BY completed_at,story_id LIMIT 100)`)
	return err
}
