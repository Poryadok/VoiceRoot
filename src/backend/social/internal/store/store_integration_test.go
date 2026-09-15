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

func startStorePostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "social-store", "")
}

func applyStoreMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	for _, name := range []string{"000001_init.up.sql", "000002_contacts.up.sql"} {
		sqlBytes, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "social_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

// TestFriendshipStoreInvitationLifecycle exercises the persistence boundary directly:
// a declined request remains visible to its sender and a new invitation reopens it.
func TestFriendshipStoreInvitationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	store := &FriendshipStore{Pool: pool}
	requester, target := uuid.New(), uuid.New()

	require.NoError(t, store.SendInvitation(ctx, requester, target))
	require.NoError(t, store.DeclineInvitation(ctx, target, requester))
	_, outgoing, err := store.ListFriendRequests(ctx, requester)
	require.NoError(t, err)
	require.Len(t, outgoing, 1)
	require.Equal(t, target, outgoing[0].TargetProfileID)
	require.Equal(t, "declined", outgoing[0].Status)

	require.NoError(t, store.SendInvitation(ctx, requester, target))
	incoming, outgoing, err := store.ListFriendRequests(ctx, target)
	require.NoError(t, err)
	require.Len(t, incoming, 1)
	require.Equal(t, requester, incoming[0].RequesterProfileID)
	require.Empty(t, outgoing)
}

// TestBlockStoreSeversAllFriendshipStates proves the account-block transaction
// removes pending, declined and accepted profile links in both directions.
func TestBlockStoreSeversAllFriendshipStates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	friends := &FriendshipStore{Pool: pool}
	blocks := &BlockStore{Pool: pool}
	accountA, accountB := uuid.New(), uuid.New()
	a1, a2, b1, b2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	require.NoError(t, friends.SendInvitation(ctx, a1, b1)) // pending
	require.NoError(t, friends.SendInvitation(ctx, a2, b2))
	require.NoError(t, friends.DeclineInvitation(ctx, b2, a2)) // declined
	acceptedRequester, acceptedTarget := uuid.New(), uuid.New()
	require.NoError(t, friends.SendInvitation(ctx, acceptedRequester, acceptedTarget))
	require.NoError(t, friends.AcceptInvitation(ctx, acceptedTarget, acceptedRequester))

	require.NoError(t, blocks.BlockAccountAndSeverFriendships(ctx, accountA, accountB,
		[]uuid.UUID{a1, a2, acceptedRequester}, []uuid.UUID{b1, b2, acceptedTarget}))
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM friendships").Scan(&count))
	require.Zero(t, count)
	blocked, err := blocks.DirectedBlockExists(ctx, accountA, accountB)
	require.NoError(t, err)
	require.True(t, blocked)
}

// TestContactStoreUpsertAndFavourite verifies the Social-owned contact state
// is idempotent and that a later favourite update is observable in its list.
func TestContactStoreUpsertAndFavourite(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	contacts := &ContactStore{Pool: pool}
	owner, contact := uuid.New(), uuid.New()

	require.NoError(t, contacts.UpsertContact(ctx, owner, contact, "phone", false))
	require.NoError(t, contacts.UpsertContact(ctx, owner, contact, "phone", true))
	favourites, err := contacts.ListFavorites(ctx, owner)
	require.NoError(t, err)
	require.Len(t, favourites, 1)
	require.Equal(t, contact, favourites[0].ContactProfileID)
	require.True(t, favourites[0].IsFavorite)

	require.NoError(t, contacts.RemoveContact(ctx, owner, contact))
	favourites, err = contacts.ListFavorites(ctx, owner)
	require.NoError(t, err)
	require.Empty(t, favourites)
}
