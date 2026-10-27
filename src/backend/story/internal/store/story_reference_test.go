package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/story/internal/store"
)

func TestDeleteStoryRetryKeepsOneExactRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated PostgreSQL")
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author, media := uuid.New(), uuid.New()
	row, err := st.CreateStory(ctx, store.CreateStoryInput{AuthorProfileID: author, Type: "photo", MediaFileID: &media, Visibility: "everyone"})
	require.NoError(t, err)
	require.NoError(t, st.DeleteStory(ctx, row.ID, author))
	require.NoError(t, st.DeleteStory(ctx, row.ID, author), "lost delete response must be safely retryable by original owner")
	require.ErrorIs(t, st.DeleteStory(ctx, row.ID, uuid.New()), store.ErrNotFound)
	_, err = st.GetStory(ctx, row.ID)
	require.ErrorIs(t, err, store.ErrNotFound)
	var n int
	var story, file uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*),min(story_id::text)::uuid,min(media_file_id::text)::uuid FROM story_media_deletion_outbox WHERE operation_id=$1`, row.ID).Scan(&n, &story, &file))
	require.Equal(t, 1, n)
	require.Equal(t, row.ID, story)
	require.Equal(t, media, file)
	_, err = st.Pool.Exec(ctx, `DELETE FROM stories WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.NoError(t, st.DeleteStory(ctx, row.ID, author), "archive cleanup cannot erase the author's successful deletion replay")
	_, err = st.Pool.Exec(ctx, `UPDATE story_deletion_receipts SET completed_at=now()-interval '31 days' WHERE story_id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, st.DeleteStory(ctx, row.ID, author), store.ErrNotFound)
	require.NoError(t, st.PruneDeletionReceipts(ctx))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM story_deletion_receipts WHERE story_id=$1`, row.ID).Scan(&n))
	require.Zero(t, n)
}

func TestMediaAcquireBeforePublicationAndCrashRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated PostgreSQL")
	}
	ctx := context.Background()
	st := startStoryStore(t)
	media, author := uuid.New(), uuid.New()
	in := store.CreateStoryInput{AuthorProfileID: author, Type: "photo", MediaFileID: &media, Visibility: "everyone"}
	var claimed uuid.UUID
	row, err := st.CreateStoryWithMedia(ctx, in, func(ctx context.Context, id uuid.UUID) error {
		claimed = id
		var visible bool
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stories WHERE id=$1)`, id).Scan(&visible))
		require.False(t, visible, "File acquisition completes before Story becomes visible")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, claimed, row.ID)
	var intents int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM story_media_reference_intents`).Scan(&intents))
	require.Zero(t, intents)
	_, err = st.CreateStoryWithMedia(ctx, in, func(_ context.Context, id uuid.UUID) error {
		claimed = id
		return errors.New("ambiguous File response")
	})
	require.Error(t, err)
	_, err = st.GetStory(ctx, claimed)
	require.ErrorIs(t, err, store.ErrNotFound)
	_, err = st.Pool.Exec(ctx, `UPDATE story_media_reference_intents SET available_at=now()-interval '1 second'`)
	require.NoError(t, err)
	var released int
	_, err = st.RecoverAbandonedMedia(ctx, func(_ context.Context, story, file uuid.UUID) error {
		require.Equal(t, claimed, story)
		require.Equal(t, media, file)
		released++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, released)
	_, err = st.RecoverAbandonedMedia(ctx, func(context.Context, uuid.UUID, uuid.UUID) error { t.Fatal("already recovered"); return nil })
	require.NoError(t, err)
}
