package testsocial

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/pkg/integrationtest"
	"voice/backend/social/internal/authctx"

	socialv1 "voice.app/voice/social/v1"
)

type fixtureAccountProfiles map[uuid.UUID][]uuid.UUID

func (m fixtureAccountProfiles) ProfileIDsForAccount(_ context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	return m[accountID], nil
}

func socialRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

// TestNewBufconnClient_BlockAccount_CascadesFixtureProfiles requires the shared
// cross-service fixture to accept the explicit User-owned account -> profile
// mapping. BlockAccount is account-scoped, but its friendship cascade needs all
// mapped profile IDs before mutating Social storage.
func TestNewBufconnClient_BlockAccount_CascadesFixtureProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	root := socialRepoRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "socialdb", filepath.Join(root, "src", "backend", "migrations", "social_db", "000001_init.up.sql"))

	blockerAccount := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	blockedAccount := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	blockerProfile := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	blockedProfile := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")

	_, err := pool.Exec(ctx, `
		INSERT INTO friendships (id, requester_profile_id, target_profile_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'accepted', now(), now())`, uuid.New(), blockerProfile, blockedProfile)
	require.NoError(t, err)

	conn, cleanup := NewBufconnClient(t, pool, fixtureAccountProfiles{
		blockerAccount: {blockerProfile},
		blockedAccount: {blockedProfile},
	})
	t.Cleanup(cleanup)
	client := socialv1.NewSocialServiceClient(conn)

	_, err = client.BlockAccount(
		metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, blockerAccount.String()),
		&socialv1.BlockAccountRequest{BlockedAccountId: blockedAccount.String()},
	)
	require.NoError(t, err)

	var blocks int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM blocks WHERE blocker_account_id = $1 AND blocked_account_id = $2`, blockerAccount, blockedAccount).Scan(&blocks)
	require.NoError(t, err)
	require.Equal(t, 1, blocks)

	var friendships int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM friendships WHERE requester_profile_id = $1 AND target_profile_id = $2`, blockerProfile, blockedProfile).Scan(&friendships)
	require.NoError(t, err)
	require.Zero(t, friendships)
}
