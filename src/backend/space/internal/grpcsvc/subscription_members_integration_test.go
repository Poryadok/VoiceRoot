package grpcsvc

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "voice.app/voice/space/v1"
)

const freeSpaceMemberCap = 50

func seedSpaceMembers(t *testing.T, ctx context.Context, client spacev1.SpaceServiceClient, ownerCtx context.Context, spaceID string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		joinerAccount := uuid.New()
		joinerProfile := uuid.New()
		inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
		require.NoError(t, err)
		joinerCtx := withAccountProfileCtx(ctx, joinerAccount, joinerProfile)
		_, err = client.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
		require.NoError(t, err, "member %d", i)
	}
}

// TestJoinByInvite_FreeTierBlocks51stMember documents free spaces cap at 50 members.
func TestJoinByInvite_FreeTierBlocks51stMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	ownerCtx := withAccountProfileCtx(ctx, ownerAccount, ownerProfile)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Free Cap"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	seedSpaceMembers(t, ctx, client, ownerCtx, spaceID, freeSpaceMemberCap-1)

	joinerAccount := uuid.New()
	joinerProfile := uuid.New()
	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = client.JoinByInvite(withAccountProfileCtx(ctx, joinerAccount, joinerProfile), &spacev1.JoinByInviteRequest{
		Code: inv.GetInvite().GetCode(),
	})
	require.Error(t, err)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestJoinByInvite_SpaceProAllows51stMember documents Space Pro raises member cap to 5000.
func TestJoinByInvite_SpaceProAllows51stMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, withSpaceProActive(true))
	t.Cleanup(cleanup)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	ownerCtx := withAccountProfileCtx(ctx, ownerAccount, ownerProfile)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Pro Cap"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	seedSpaceMembers(t, ctx, client, ownerCtx, spaceID, freeSpaceMemberCap-1)

	joinerAccount := uuid.New()
	joinerProfile := uuid.New()
	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = client.JoinByInvite(withAccountProfileCtx(ctx, joinerAccount, joinerProfile), &spacev1.JoinByInviteRequest{
		Code: inv.GetInvite().GetCode(),
	})
	require.NoError(t, err)
	require.True(t, spaceHasProEntitlement(ctx, pool, spaceID), "space must have active Space Pro entitlement")
}

// TestJoinByInvite_CancelledSpaceProBlocksNewJoinOverFreeCap documents post-cancel join block above 50.
func TestJoinByInvite_CancelledSpaceProBlocksNewJoinOverFreeCap(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, withSpaceProActive(false), withSpaceProCancelledOverCap(true))
	t.Cleanup(cleanup)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	ownerCtx := withAccountProfileCtx(ctx, ownerAccount, ownerProfile)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: fmt.Sprintf("Post Cancel %d", freeSpaceMemberCap)})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	seedSpaceMembers(t, ctx, client, ownerCtx, spaceID, freeSpaceMemberCap)

	_, err = pool.Exec(ctx, `UPDATE space_subscriptions SET status = 'cancelled', updated_at = now() WHERE space_id = $1::uuid`, spaceID)
	require.NoError(t, err)

	joinerAccount := uuid.New()
	joinerProfile := uuid.New()
	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = client.JoinByInvite(withAccountProfileCtx(ctx, joinerAccount, joinerProfile), &spacev1.JoinByInviteRequest{
		Code: inv.GetInvite().GetCode(),
	})
	require.Error(t, err)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// withSpaceProActive seeds active Space Pro entitlement when a space is created in tests.
func withSpaceProActive(active bool) spaceServerOption {
	return func(s *SpaceGRPC) {
		s.SeedSpaceProActive = active
	}
}

// withSpaceProCancelledOverCap enables Space Pro during setup so the space can grow past 50, then tests post-cancel joins.
func withSpaceProCancelledOverCap(overCap bool) spaceServerOption {
	return func(s *SpaceGRPC) {
		s.SeedSpaceProActive = overCap
	}
}

func spaceHasProEntitlement(ctx context.Context, pool *pgxpool.Pool, spaceID string) bool {
	var exists bool
	_ = pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM space_subscriptions
  WHERE space_id = $1::uuid AND status IN ('active', 'grace_period')
)`, spaceID).Scan(&exists)
	return exists
}
