package store

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func searchModuleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func TestMessageSearchStore_UpsertAndFTSSearch_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)

	chatA := uuid.New()
	chatB := uuid.New()
	msgMatch := uuid.New()
	msgOtherChat := uuid.New()
	msgNoMatch := uuid.New()
	sender := uuid.New()

	st := NewMessageSearchStore(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	require.NoError(t, st.Upsert(ctx, MessageDocument{
		MessageID:       msgMatch,
		ChatID:          chatA,
		SenderProfileID: sender,
		Body:            "PostgreSQL full text search works",
		CreatedAt:       now,
	}))
	require.NoError(t, st.Upsert(ctx, MessageDocument{
		MessageID:       msgOtherChat,
		ChatID:          chatB,
		SenderProfileID: sender,
		Body:            "PostgreSQL full text search works",
		CreatedAt:       now,
	}))
	require.NoError(t, st.Upsert(ctx, MessageDocument{
		MessageID:       msgNoMatch,
		ChatID:          chatA,
		SenderProfileID: sender,
		Body:            "unrelated content",
		CreatedAt:       now,
	}))

	t.Run("fts match returns snippet and score", func(t *testing.T) {
		hits, next, err := st.SearchInChat(ctx, chatA, "full text", nil, 20)
		require.NoError(t, err)
		require.Empty(t, next)
		require.Len(t, hits, 1)
		require.Equal(t, msgMatch, hits[0].MessageID)
		require.Equal(t, chatA, hits[0].ChatID)
		require.Contains(t, hits[0].Snippet, "<b>full</b>")
		require.Contains(t, hits[0].Snippet, "<b>text</b>")
		require.Greater(t, hits[0].Score, 0.0)
	})

	t.Run("chat_id filter scopes in-chat search", func(t *testing.T) {
		hits, _, err := st.SearchInChat(ctx, chatB, "full text", nil, 20)
		require.NoError(t, err)
		require.Len(t, hits, 1)
		require.Equal(t, msgOtherChat, hits[0].MessageID)
		require.Equal(t, chatB, hits[0].ChatID)
	})
}

func TestMessageSearchStore_DeleteRemovesFromIndex_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)

	chatID := uuid.New()
	msgID := uuid.New()
	st := NewMessageSearchStore(pool)
	now := time.Now().UTC()

	require.NoError(t, st.Upsert(ctx, MessageDocument{
		MessageID:       msgID,
		ChatID:          chatID,
		SenderProfileID: uuid.New(),
		Body:            "delete me from index",
		CreatedAt:       now,
	}))
	require.NoError(t, st.Delete(ctx, msgID))

	hits, _, err := st.SearchInChat(ctx, chatID, "delete", nil, 20)
	require.NoError(t, err)
	require.Empty(t, hits)
}

func TestMessageSearchStore_SearchPaginationDefault20_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)

	chatID := uuid.New()
	st := NewMessageSearchStore(pool)
	now := time.Now().UTC()
	for i := 0; i < 25; i++ {
		require.NoError(t, st.Upsert(ctx, MessageDocument{
			MessageID:       uuid.New(),
			ChatID:          chatID,
			SenderProfileID: uuid.New(),
			Body:            "pagination token alpha",
			CreatedAt:       now.Add(time.Duration(i) * time.Millisecond),
		}))
	}

	first, next, err := st.SearchInChat(ctx, chatID, "pagination", nil, 0)
	require.NoError(t, err)
	require.Len(t, first, 20, "default page_size must be 20 when limit is 0")
	require.NotEmpty(t, next)

	second, nextAfter, err := st.SearchInChat(ctx, chatID, "pagination", &next, 20)
	require.NoError(t, err)
	require.NotEmpty(t, second)
	require.Equal(t, 25, len(first)+len(second))
	if len(second) < 5 {
		require.NotEmpty(t, nextAfter)
	}
}
