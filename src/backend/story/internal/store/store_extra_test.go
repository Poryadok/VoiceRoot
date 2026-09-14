package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/story/internal/store"
)

func TestListActiveStories_andListViewers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author := uuid.New()
	viewer := uuid.New()
	text := "active"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: author,
		Type:            "text",
		TextContent:     &text,
		Visibility:      "everyone",
	})
	require.NoError(t, err)

	active, err := st.ListActiveStories(ctx, 10)
	require.NoError(t, err)
	require.NotEmpty(t, active)

	byAuthor, err := st.ListActiveStoriesByAuthor(ctx, author)
	require.NoError(t, err)
	require.Len(t, byAuthor, 1)

	require.NoError(t, st.MarkViewed(ctx, row.ID, viewer, false))
	viewers, err := st.ListViewers(ctx, row.ID)
	require.NoError(t, err)
	require.Contains(t, viewers, viewer)
}

func TestPurgeArchivedStories(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author := uuid.New()
	text := "purge"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: author,
		Type:            "text",
		TextContent:     &text,
		Visibility:      "friends",
	})
	require.NoError(t, err)
	_, err = st.MarkExpiredStories(ctx, row.ExpiresAt.Add(1))
	require.NoError(t, err)
	got, err := st.GetStory(ctx, row.ID)
	require.NoError(t, err)
	n, err := st.PurgeArchivedStories(ctx, got.ArchivedUntil.Add(1))
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}

func TestPurgeArchivedStories_retainsHighlightedStoryUntilUnlinked(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	profile := uuid.New()
	mediaID := uuid.New()
	text := "keep this highlight"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: profile,
		Type:            "photo",
		MediaFileID:     &mediaID,
		TextContent:     &text,
		Visibility:      "friends",
	})
	require.NoError(t, err)

	_, err = st.MarkExpiredStories(ctx, row.ExpiresAt.Add(time.Second))
	require.NoError(t, err)
	highlight, err := st.CreateHighlight(ctx, profile, "Permanent", "everyone")
	require.NoError(t, err)
	require.NoError(t, st.AddToHighlight(ctx, highlight.ID, profile, row.ID))

	purgeAt := row.ArchivedUntil.Add(time.Second)
	candidates, err := st.ListArchivedStoriesForPurge(ctx, purgeAt)
	require.NoError(t, err)
	require.Empty(t, candidates, "highlight membership must retain story media past archive retention")
	n, err := st.PurgeArchivedStories(ctx, purgeAt)
	require.NoError(t, err)
	require.Zero(t, n)
	retained, err := st.GetStory(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, mediaID, *retained.MediaFileID)
	highlights, err := st.GetHighlights(ctx, profile)
	require.NoError(t, err)
	require.Contains(t, highlights[0].StoryIDs, row.ID, "purge must not cascade-delete a retained highlight link")

	require.NoError(t, st.RemoveFromHighlight(ctx, highlight.ID, profile, row.ID))
	candidates, err = st.ListArchivedStoriesForPurge(ctx, purgeAt)
	require.NoError(t, err)
	require.Len(t, candidates, 1, "unlinked expired story must become eligible for normal archive cleanup")
	require.Equal(t, row.ID, candidates[0].StoryID)
	require.NotNil(t, candidates[0].MediaFileID)
	require.Equal(t, mediaID, *candidates[0].MediaFileID)
	n, err = st.PurgeArchivedStories(ctx, purgeAt)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	_, err = st.GetStory(ctx, row.ID)
	require.Error(t, err)
}
