package indexer_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
	"voice/backend/search/internal/deps"
	"voice/backend/search/internal/indexer"
	"voice/backend/search/internal/store"
)

func searchModuleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

// ENC-12 / encryption.md: ReindexChat must not upsert E2E bodies into message_search_documents
// (server-side search must never index ciphertext).
func TestReindexChat_SkipsE2EBodies_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)

	chatID := uuid.New()
	plainID := uuid.New()
	e2eID := uuid.New()
	sender := uuid.New()
	created := time.Now().UTC().Truncate(time.Microsecond)

	const plainBody = "plaintext searchable uniquephrase"
	const e2eBody = "ciphertext secretzyx neverindex"

	lister := &stubMessageLister{
		pages: [][]deps.MessageRow{
			{
				{ID: plainID, SenderProfileID: sender, Body: plainBody, CreatedAt: created, IsE2E: false},
				{ID: e2eID, SenderProfileID: sender, Body: e2eBody, CreatedAt: created, IsE2E: true},
			},
		},
	}
	msgStore := store.NewMessageSearchStore(pool)

	require.NoError(t, indexer.ReindexChat(ctx, chatID, lister, msgStore))

	plainHits, _, err := msgStore.SearchInChat(ctx, chatID, "uniquephrase", nil, 20)
	require.NoError(t, err)
	require.Len(t, plainHits, 1)
	require.Equal(t, plainID, plainHits[0].MessageID)

	e2eHits, _, err := msgStore.SearchInChat(ctx, chatID, "secretzyx", nil, 20)
	require.NoError(t, err)
	require.Empty(t, e2eHits, "E2E ciphertext must not be searchable after reindex")
}
