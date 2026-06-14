package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func chatRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startChatDBForStoreTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "chatdb", "")
}

func applyChatMigrationsForStoreTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root := chatRepoRoot(t)
	for _, name := range []string{"000001_init.up.sql", "000002_dm_requests.up.sql", "000003_groups.up.sql", "000005_thread_settings.up.sql", "000006_e2e_enabled.up.sql"} {
		sqlBytes, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "chat_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

// TestCreateGroupChat_OwnerMembership documents creator is persisted as owner in main inbox.
func TestCreateGroupChat_OwnerMembership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	owner := uuid.New()
	row, err := store.CreateGroupChat(ctx, owner, "Store group")
	require.NoError(t, err)
	require.Equal(t, "group", row.Type)
	require.Equal(t, "Store group", *row.Name)

	role, err := store.GetMemberRole(ctx, row.ID, owner)
	require.NoError(t, err)
	require.Equal(t, "owner", role)

	n, err := store.CountChatMembers(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

// TestAddGroupMembers_MinThreeMembers documents text-chat.md minimum before commit.
func TestAddGroupMembers_MinThreeMembers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	owner := uuid.New()
	invitee := uuid.New()
	row, err := store.CreateGroupChat(ctx, owner, "Min members")
	require.NoError(t, err)

	_, err = store.AddGroupMembers(ctx, row.ID, []uuid.UUID{invitee})
	require.ErrorIs(t, err, ErrGroupTooFewMembers)
}

// TestAddGroupMembers_MemberLimit documents chat-service.md 500 cap at store layer.
func TestAddGroupMembers_MemberLimit(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	owner := uuid.New()
	row, err := store.CreateGroupChat(ctx, owner, "Full")
	require.NoError(t, err)

	ids := make([]uuid.UUID, 0, GroupMemberLimit-1)
	for i := 0; i < GroupMemberLimit-2; i++ {
		ids = append(ids, uuid.New())
	}
	_, err = store.AddGroupMembers(ctx, row.ID, ids)
	require.NoError(t, err)

	_, err = store.AddGroupMembers(ctx, row.ID, []uuid.UUID{uuid.New(), uuid.New()})
	require.ErrorIs(t, err, ErrGroupMemberLimit)
}

// TestAddGroupMembers_IdempotentSkipExisting avoids duplicate rows for same profile.
func TestAddGroupMembers_IdempotentSkipExisting(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	owner := uuid.New()
	a, b := uuid.New(), uuid.New()
	row, err := store.CreateGroupChat(ctx, owner, "Dedup")
	require.NoError(t, err)

	added, err := store.AddGroupMembers(ctx, row.ID, []uuid.UUID{a, b})
	require.NoError(t, err)
	require.Len(t, added, 2)

	added, err = store.AddGroupMembers(ctx, row.ID, []uuid.UUID{a})
	require.NoError(t, err)
	require.Empty(t, added)

	n, err := store.CountChatMembers(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 3, n)
}

// TestUpdateGroupChat_PersistsAvatar documents avatar_url persistence in chat_db.
func TestUpdateGroupChat_PersistsAvatar(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	owner := uuid.New()
	row, err := store.CreateGroupChat(ctx, owner, "Avatar")
	require.NoError(t, err)

	avatar := "https://cdn.voice.gg/groups/store.webp"
	updated, err := store.UpdateGroupChat(ctx, row.ID, nil, &avatar, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.AvatarURL)
	require.Equal(t, avatar, *updated.AvatarURL)

	loaded, err := store.FindChatByID(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, avatar, *loaded.AvatarURL)
}

// TestRemoveGroupMember_DeletesRow documents kick at persistence layer.
func TestRemoveGroupMember_DeletesRow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	owner := uuid.New()
	target := uuid.New()
	extra := uuid.New()
	row, err := store.CreateGroupChat(ctx, owner, "Kick store")
	require.NoError(t, err)
	_, err = store.AddGroupMembers(ctx, row.ID, []uuid.UUID{target, extra})
	require.NoError(t, err)

	require.NoError(t, store.RemoveGroupMember(ctx, row.ID, target))
	role, err := store.GetMemberRole(ctx, row.ID, target)
	require.NoError(t, err)
	require.Empty(t, role)
}
