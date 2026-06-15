package store_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/manifest"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func migrationSQL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", "000001_init.up.sql")
	b, err := os.ReadFile(root)
	require.NoError(t, err)
	return string(b)
}

func startBotStore(t *testing.T) *store.BotStore {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "botdb", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	return &store.BotStore{Pool: pool}
}

func TestCreateBot_andTokenLookup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	owner := uuid.New()
	row, plain, err := st.CreateBot(ctx, owner, "PingBot", "pong", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.Nil)
	require.NoError(t, err)
	require.NotEmpty(t, plain)
	got, err := st.GetBotByTokenHash(ctx, store.HashToken(plain))
	require.NoError(t, err)
	require.Equal(t, row.ID, got.ID)
}

func TestWhitelist_blocksUnknownChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	owner := uuid.New()
	profile := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "PingBot", "", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.Nil)
	require.NoError(t, err)
	spaceID := uuid.New()
	chatID := uuid.New()
	_, err = st.InstallInSpace(ctx, row.ID, spaceID, profile, []uuid.UUID{chatID})
	require.NoError(t, err)
	ok, err := st.IsChatWhitelisted(ctx, row.ID, chatID)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = st.IsChatWhitelisted(ctx, row.ID, uuid.New())
	require.NoError(t, err)
	require.False(t, ok)
}

func TestApplyManifest_registersPingCommand(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "PingBot", "", `[]`, uuid.Nil)
	require.NoError(t, err)
	doc, _, err := manifest.ParseYAML(`
name: PingBot
description: pong
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`)
	require.NoError(t, err)
	require.NoError(t, st.ApplyManifest(ctx, row.ID, doc))
	cmds, err := st.ListCommands(ctx, row.ID)
	require.NoError(t, err)
	require.Len(t, cmds, 1)
	require.Equal(t, "ping", cmds[0].Name)
}
