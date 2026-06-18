package store_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/story/internal/store"
	"voice/backend/pkg/integrationtest"
)

func migrationSQL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "story_db")
	b1, err := os.ReadFile(filepath.Join(dir, "000001_init.up.sql"))
	require.NoError(t, err)
	b2, err := os.ReadFile(filepath.Join(dir, "000002_visibility_audience.up.sql"))
	require.NoError(t, err)
	return string(b1) + "\n" + string(b2)
}

func startStoryStore(t *testing.T) *store.StoryStore {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storydb", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	return &store.StoryStore{Pool: pool}
}

func TestCreateStory_andGetStory(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author := uuid.New()
	text := "hello story"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: author,
		Type:          "text",
		TextContent:   &text,
		Visibility:    "friends",
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, row.ID)
	require.Equal(t, author, row.AuthorProfileID)
	require.Equal(t, "text", row.Type)
	require.WithinDuration(t, time.Now().Add(24*time.Hour), row.ExpiresAt, 2*time.Minute)

	got, err := st.GetStory(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, row.ID, got.ID)
	require.Equal(t, author, got.AuthorProfileID)
}

func TestDeleteStory_removesFromActive(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author := uuid.New()
	text := "delete me"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: author,
		Type:          "text",
		TextContent:   &text,
		Visibility:    "friends",
	})
	require.NoError(t, err)

	require.NoError(t, st.DeleteStory(ctx, row.ID, author))

	_, err = st.GetStory(ctx, row.ID)
	require.Error(t, err)
}

func TestMarkViewed_incrementsViewCount(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author := uuid.New()
	viewer := uuid.New()
	text := "views"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: author,
		Type:          "text",
		TextContent:   &text,
		Visibility:    "friends",
	})
	require.NoError(t, err)

	require.NoError(t, st.MarkViewed(ctx, row.ID, viewer, false))

	got, err := st.GetStory(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 1, got.ViewCount)
}

func TestReactToStory_upsertsEmoji(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author := uuid.New()
	reactor := uuid.New()
	text := "react"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: author,
		Type:          "text",
		TextContent:   &text,
		Visibility:    "friends",
	})
	require.NoError(t, err)

	require.NoError(t, st.ReactToStory(ctx, row.ID, reactor, "🔥"))
	require.NoError(t, st.ReactToStory(ctx, row.ID, reactor, "❤️"))
}

func TestHighlights_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	profile := uuid.New()
	text := "highlight me"
	storyRow, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: profile,
		Type:          "text",
		TextContent:   &text,
		Visibility:    "friends",
	})
	require.NoError(t, err)

	hl, err := st.CreateHighlight(ctx, profile, "Best plays", "everyone")
	require.NoError(t, err)
	require.Equal(t, "Best plays", hl.Name)

	updated, err := st.UpdateHighlight(ctx, hl.ID, profile, "Clutch moments", "")
	require.NoError(t, err)
	require.Equal(t, "Clutch moments", updated.Name)

	require.NoError(t, st.AddToHighlight(ctx, hl.ID, profile, storyRow.ID))

	list, err := st.GetHighlights(ctx, profile)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Contains(t, list[0].StoryIDs, storyRow.ID)

	require.NoError(t, st.RemoveFromHighlight(ctx, hl.ID, profile, storyRow.ID))
	require.NoError(t, st.DeleteHighlight(ctx, hl.ID, profile))

	list, err = st.GetHighlights(ctx, profile)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestListArchive_afterExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author := uuid.New()
	text := "archive"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: author,
		Type:          "text",
		TextContent:   &text,
		Visibility:    "friends",
	})
	require.NoError(t, err)

	n, err := st.MarkExpiredStories(ctx, row.ExpiresAt.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	archive, err := st.ListArchive(ctx, author)
	require.NoError(t, err)
	require.Len(t, archive, 1)
	require.Equal(t, row.ID, archive[0].ID)
}

func TestMarkExpiredStories_transitionsState(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStoryStore(t)
	author := uuid.New()
	text := "ttl"
	row, err := st.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: author,
		Type:          "text",
		TextContent:   &text,
		Visibility:    "friends",
	})
	require.NoError(t, err)
	require.Nil(t, row.ExpiredAt)

	n, err := st.MarkExpiredStories(ctx, row.ExpiresAt.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	got, err := st.GetStory(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ExpiredAt)
}
