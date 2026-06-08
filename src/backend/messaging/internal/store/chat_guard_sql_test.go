package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSQLChatGuard_nilPool(t *testing.T) {
	t.Parallel()
	var g *SQLChatGuard
	require.NoError(t, g.EnsureMember(context.Background(), uuid.New(), uuid.New()))
}

func TestSQLChatGuard_membership(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedChatSchema(t, ctx, pool)
	g := &SQLChatGuard{Pool: pool}

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds) VALUES ($1, 'dm', $2, 0)
`, chatID, profA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')
`, chatID, profA)
	require.NoError(t, err)

	err = g.EnsureMember(ctx, chatID, profA)
	require.NoError(t, err)
	err = g.EnsureMember(ctx, chatID, profB)
	require.ErrorIs(t, err, ErrNotChatMember)

	_, err = g.DMOtherProfileID(ctx, chatID, profA)
	require.Error(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')
`, chatID, profB)
	require.NoError(t, err)

	peer, err := g.DMOtherProfileID(ctx, chatID, profA)
	require.NoError(t, err)
	require.Equal(t, profB, peer)

	var nilGuard *SQLChatGuard
	_, err = nilGuard.DMOtherProfileID(ctx, chatID, profA)
	require.Error(t, err)
}

func TestSQLChatGuard_DMOtherProfileID_dbErrorOnPeerLookup(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedChatSchema(t, ctx, pool)
	g := &SQLChatGuard{Pool: pool}

	chatID := uuid.New()
	profA := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds) VALUES ($1, 'dm', $2, 0)
`, chatID, profA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, chatID, profA)
	require.NoError(t, err)
	pool.Close()

	_, err = g.DMOtherProfileID(ctx, chatID, profA)
	require.Error(t, err)
}

func TestSQLChatGuard_dbError(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedChatSchema(t, ctx, pool)
	pool.Close()
	g := &SQLChatGuard{Pool: pool}
	err := g.EnsureMember(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
}
