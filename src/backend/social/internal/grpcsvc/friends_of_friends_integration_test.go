package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	socialv1 "voice.app/voice/social/v1"
)

func insertAcceptedFriendship(t *testing.T, ctx context.Context, pool *pgxpool.Pool, a, b uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO friendships (id, requester_profile_id, target_profile_id, status, created_at, updated_at)
VALUES ($1, $2, $3, 'accepted', NOW(), NOW())`, uuid.New(), a, b)
	require.NoError(t, err)
}

// TestAreFriendsOfFriends_OneHop documents privacy.md: single-level FoF (A—C—B, not direct friends).
func TestAreFriendsOfFriends_OneHop(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	a, b, c := uuid.New(), uuid.New(), uuid.New()
	insertAcceptedFriendship(t, ctx, pool, a, c)
	insertAcceptedFriendship(t, ctx, pool, c, b)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	direct, err := client.AreFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: a.String(),
		ProfileIdB: b.String(),
	})
	require.NoError(t, err)
	require.False(t, direct.GetFriends())

	fof, err := client.AreFriendsOfFriends(ctx, &socialv1.AreFriendsOfFriendsRequest{
		ProfileIdA: a.String(),
		ProfileIdB: b.String(),
	})
	require.NoError(t, err)
	require.True(t, fof.GetFriends())

	list, err := client.GetFriendsOfFriends(ctx, &socialv1.GetFriendsOfFriendsRequest{
		ProfileId: a.String(),
	})
	require.NoError(t, err)
	require.Contains(t, list.GetProfileIdList().GetProfileIds(), b.String())
}
