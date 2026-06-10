package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestCreateInvite_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profile, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Invites"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	resp, err := client.CreateInvite(ctx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetInvite().GetCode())
	require.Equal(t, spaceID, resp.GetInvite().GetSpaceId())
	require.Equal(t, profile.String(), resp.GetInvite().GetCreatorProfileId())
}

func TestCreateInvite_NonOwnerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	strangerAccount, strangerProfile := uuid.New(), uuid.New()
	strangerCtx := withAccountProfileCtx(context.Background(), strangerAccount, strangerProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Private"})
	require.NoError(t, err)

	_, err = client.CreateInvite(strangerCtx, &spacev1.CreateInviteRequest{
		SpaceId: created.GetSpace().GetId(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestJoinByInvite_AddsMemberAndListsSpace(t *testing.T) {
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

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Join Me"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)

	joined, err := client.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)
	require.Equal(t, spaceID, joined.GetSpaceMembership().GetSpaceId())
	require.Equal(t, joinerProfile.String(), joined.GetSpaceMembership().GetProfileId())

	list, err := client.ListMySpaces(joinerCtx, &spacev1.ListMySpacesRequest{Page: &commonv1.CursorPageRequest{}})
	require.NoError(t, err)
	require.Len(t, list.GetSpaceList().GetSpaces(), 1)
}

func TestJoinByInvite_IdempotentForMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Again"})
	require.NoError(t, err)
	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{
		SpaceId: created.GetSpace().GetId(),
	})
	require.NoError(t, err)

	first, err := client.JoinByInvite(ownerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)
	second, err := client.JoinByInvite(ownerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)
	require.Equal(t, first.GetSpaceMembership().GetJoinedAt().AsTime(), second.GetSpaceMembership().GetJoinedAt().AsTime())
}

func TestGetInvite_ByCode(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Preview"})
	require.NoError(t, err)
	inv, err := client.CreateInvite(ctx, &spacev1.CreateInviteRequest{
		SpaceId: created.GetSpace().GetId(),
	})
	require.NoError(t, err)

	got, err := client.GetInvite(ctx, &spacev1.GetInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)
	require.Equal(t, inv.GetInvite().GetId(), got.GetInvite().GetId())
}

func TestRevokeInvite_BlocksJoin(t *testing.T) {
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

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Revoke"})
	require.NoError(t, err)
	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{
		SpaceId: created.GetSpace().GetId(),
	})
	require.NoError(t, err)

	_, err = client.RevokeInvite(ownerCtx, &spacev1.RevokeInviteRequest{InviteId: inv.GetInvite().GetId()})
	require.NoError(t, err)

	_, err = client.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestCreateInvite_DegradedWhenEventsFail(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, withSpaceEventsPublisher(errSpaceEvents{}))
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Degraded"})
	require.NoError(t, err)

	resp, err := client.CreateInvite(ctx, &spacev1.CreateInviteRequest{
		SpaceId: created.GetSpace().GetId(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetInvite().GetCode())
}

func TestListInvites_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "List Invites"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	_, err = client.CreateInvite(ctx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)

	list, err := client.ListInvites(ctx, &spacev1.ListInvitesRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Len(t, list.GetInviteList().GetInvites(), 1)
}

func TestJoinByInvite_Expired(t *testing.T) {
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

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Expired"})
	require.NoError(t, err)
	past := time.Now().UTC().Add(-time.Hour)
	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{
		SpaceId:   created.GetSpace().GetId(),
		ExpiresAt: timestamppb.New(past),
	})
	require.NoError(t, err)

	_, err = client.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
