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

// TestJoinSpace_BlockCheckNotConfigured documents OPERATIONS.md fail-closed when Social/User S2S is unwired.
func TestJoinSpace_BlockCheckNotConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	joinerAccount, joinerProfile := uuid.New(), uuid.New()
	joinerCtx := withAccountProfileCtx(context.Background(), joinerAccount, joinerProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, skipJoinBlockDefaults())
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

	_, err = client.JoinSpace(joinerCtx, &spacev1.JoinSpaceRequest{SpaceId: spaceID})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestJoinByInvite_BlockCheckNotConfigured documents fail-closed on invite join when block deps are unwired.
func TestJoinByInvite_BlockCheckNotConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	joinerAccount, joinerProfile := uuid.New(), uuid.New()
	joinerCtx := withAccountProfileCtx(context.Background(), joinerAccount, joinerProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, skipJoinBlockDefaults())
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Invite block deps"})
	require.NoError(t, err)

	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: created.GetSpace().GetId()})
	require.NoError(t, err)

	_, err = client.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
