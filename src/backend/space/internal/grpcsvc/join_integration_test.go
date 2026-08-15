package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "voice.app/voice/space/v1"
)

func TestJoinSpace_PublicNoneRequirement(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	joinerAccount, joinerProfile := uuid.New(), uuid.New()
	joinerCtx := withAccountProfileCtx(context.Background(), joinerAccount, joinerProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{
		Name:        "Public Plaza",
		Visibility:  "public",
		Description: "open",
	})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	_, err = client.UpdateSpace(ownerCtx, &spacev1.UpdateSpaceRequest{
		SpaceId:          spaceID,
		EntryRequirement: strPtr("none"),
	})
	require.NoError(t, err)

	joined, err := client.JoinSpace(joinerCtx, &spacev1.JoinSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, spaceID, joined.GetSpaceMembership().GetSpaceId())
	require.Equal(t, joinerProfile.String(), joined.GetSpaceMembership().GetProfileId())
}

func TestJoinSpace_EntryRequirementBlocksPublicJoin(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	joinerAccount, joinerProfile := uuid.New(), uuid.New()
	joinerCtx := withAccountProfileCtx(context.Background(), joinerAccount, joinerProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{
		Name:       "Gated",
		Visibility: "public",
	})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	_, err = client.UpdateSpace(ownerCtx, &spacev1.UpdateSpaceRequest{
		SpaceId:          spaceID,
		EntryRequirement: strPtr("manual"),
	})
	require.NoError(t, err)

	_, err = client.JoinSpace(joinerCtx, &spacev1.JoinSpaceRequest{SpaceId: spaceID})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestLeaveSpace_RemovesMembership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	joinerAccount, joinerProfile := uuid.New(), uuid.New()
	joinerCtx := withAccountProfileCtx(context.Background(), joinerAccount, joinerProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Leave test"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = client.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)

	_, err = client.LeaveSpace(joinerCtx, &spacev1.LeaveSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)

	_, err = client.GetSpace(joinerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
