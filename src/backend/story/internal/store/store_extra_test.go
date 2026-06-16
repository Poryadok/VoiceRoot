package store_test

import (
	"context"
	"testing"

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
		Type:          "text",
		TextContent:   &text,
		Visibility:    "everyone",
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
		Type:          "text",
		TextContent:   &text,
		Visibility:    "friends",
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
