package store_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func botPresenceMigrationSQL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db")
	initSQL, err := os.ReadFile(filepath.Join(dir, "000001_init.up.sql"))
	require.NoError(t, err)
	presenceSQL, err := os.ReadFile(filepath.Join(dir, "000002_bot_presence.up.sql"))
	require.NoError(t, err)
	return string(initSQL) + "\n" + string(presenceSQL)
}

func startBotStoreWithPresence(t *testing.T) *store.BotStore {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "botdb_presence", "")
	_, err := pool.Exec(ctx, botPresenceMigrationSQL(t))
	require.NoError(t, err)
	return &store.BotStore{Pool: pool}
}

// TestIncrementDailyChatCreates_concurrent proves Postgres upsert is atomic under concurrent writers.
func TestIncrementDailyChatCreates_concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "LimitBot", "", `["TEXT_CHAT_CREATE_IN_SPACE"]`, uuid.Nil)
	require.NoError(t, err)

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			_, incErr := st.IncrementDailyChatCreates(ctx, row.ID)
			require.NoError(t, incErr)
		}()
	}
	wg.Wait()

	count, err := st.IncrementDailyChatCreates(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, workers+1, count, "atomic upsert must count every increment")
}
