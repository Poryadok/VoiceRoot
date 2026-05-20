package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/social/internal/authctx"

	commonv1 "voice.app/voice/common/v1"
	socialv1 "voice.app/voice/social/v1"
)

func withAccountCtx(ctx context.Context, accountID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
}

func startSocialPostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "socialdb", "")
}

func applySocialMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "social_db", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}

func TestBlockFlow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()

	// Missing account in metadata
	_, err := client.BlockAccount(withProfileCtx(ctx, profA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// Self block
	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accA.String()})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	// A blocks B
	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)
	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)

	lb, err := client.ListBlocked(withAccountCtx(ctx, accA), &socialv1.ListBlockedRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, lb.GetBlockedList().GetBlocked(), 1)
	require.Equal(t, accB.String(), lb.GetBlockedList().GetBlocked()[0].GetBlockedAccountId())

	_, err = client.UnblockAccount(withAccountCtx(ctx, accA), &socialv1.UnblockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)
	_, err = client.UnblockAccount(withAccountCtx(ctx, accA), &socialv1.UnblockAccountRequest{BlockedAccountId: accB.String()})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))

	lb2, err := client.ListBlocked(withAccountCtx(ctx, accA), &socialv1.ListBlockedRequest{})
	require.NoError(t, err)
	require.Empty(t, lb2.GetBlockedList().GetBlocked())

	// Internal IsBlocked: ordered pair (blocker, blocked); no user metadata required.
	rNo, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: accA.String(),
		AccountIdB: accB.String(),
	})
	require.NoError(t, err)
	require.False(t, rNo.GetBlocked())

	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)
	rAB, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: accA.String(),
		AccountIdB: accB.String(),
	})
	require.NoError(t, err)
	require.True(t, rAB.GetBlocked())
	rBA, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: accB.String(),
		AccountIdB: accA.String(),
	})
	require.NoError(t, err)
	require.False(t, rBA.GetBlocked())
}

func TestListBlocked_InvalidCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	acc := uuid.New()
	_, err := client.ListBlocked(withAccountCtx(ctx, acc), &socialv1.ListBlockedRequest{
		Page: &commonv1.CursorPageRequest{Cursor: "garbage", PageSize: 5},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// dmPairBlocked is the account-level predicate Chat (CreateDM) should use: either party
// blocking the other forbids opening a DM between the two accounts. Social exposes only
// directed IsBlocked; callers OR both directions (see SocialGRPC.IsBlocked).
func dmPairBlocked(t *testing.T, client socialv1.SocialServiceClient, ctx context.Context, accA, accB uuid.UUID) bool {
	t.Helper()
	rAB, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: accA.String(),
		AccountIdB: accB.String(),
	})
	require.NoError(t, err)
	rBA, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: accB.String(),
		AccountIdB: accA.String(),
	})
	require.NoError(t, err)
	return rAB.GetBlocked() || rBA.GetBlocked()
}

func TestIsBlocked_DMGate_SingleDirectionBlock_ForbidsEitherInitiator(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accA := uuid.New()
	accB := uuid.New()

	require.False(t, dmPairBlocked(t, client, ctx, accA, accB))

	_, err := client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)

	// Directed checks: only A blocked B.
	rAB, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{AccountIdA: accA.String(), AccountIdB: accB.String()})
	require.NoError(t, err)
	require.True(t, rAB.GetBlocked())
	rBA, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{AccountIdA: accB.String(), AccountIdB: accA.String()})
	require.NoError(t, err)
	require.False(t, rBA.GetBlocked())

	// DM gate: either user could have initiated — both directions must be consulted.
	require.True(t, dmPairBlocked(t, client, ctx, accA, accB), "A blocked B: DM must be denied regardless of initiator")
	require.True(t, dmPairBlocked(t, client, ctx, accB, accA), "symmetric argument order for helper")
}

func TestIsBlocked_DMGate_ReverseBlock_ForbidsEitherInitiator(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accA := uuid.New()
	accB := uuid.New()

	_, err := client.BlockAccount(withAccountCtx(ctx, accB), &socialv1.BlockAccountRequest{BlockedAccountId: accA.String()})
	require.NoError(t, err)

	require.True(t, dmPairBlocked(t, client, ctx, accA, accB))
}

func TestIsBlocked_SelfPair_NotBlocked(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	acc := uuid.New()
	r, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: acc.String(),
		AccountIdB: acc.String(),
	})
	require.NoError(t, err)
	require.False(t, r.GetBlocked())
}

func TestIsBlocked_InvalidUUIDs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: "",
		AccountIdB: uuid.New().String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
