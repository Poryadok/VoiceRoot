package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"
	roletest "voice/backend/role/testutil"
	"voice/backend/space/internal/authctx"

	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

func startSharedRoleClient(t *testing.T) (rolev1.RoleServiceClient, func()) {
	t.Helper()
	ctx := context.Background()
	pool := roletest.StartRoleDB(t, ctx)
	roletest.ApplyRoleMigrations(t, ctx, pool)
	return roletest.StartRoleGRPC(t, pool)
}

// TestCreateSpace_BootstrapSystemRoles documents spaces.md: CreateSpace seeds role hierarchy via Role Service.
func TestCreateSpace_BootstrapSystemRoles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	roleClient, roleCleanup := startSharedRoleClient(t)
	t.Cleanup(roleCleanup)
	spaceClient, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roleClient))
	t.Cleanup(cleanup)

	created, err := spaceClient.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Roles QA"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	list, err := roleClient.ListRoles(ctx, &rolev1.ListRolesRequest{SpaceId: spaceID})
	require.NoError(t, err)
	roles := list.GetRoleList().GetRoles()
	require.Len(t, roles, 5)
	require.Equal(t, permissions.RoleOwner, roles[0].GetName())
}

// TestJoinByInvite_AssignsMemberRole documents default Member role on join.
func TestJoinByInvite_AssignsMemberRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	joinerAccount, joinerProfile := uuid.New(), uuid.New()
	joinerCtx := withAccountProfileCtx(context.Background(), joinerAccount, joinerProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	roleClient, roleCleanup := startSharedRoleClient(t)
	t.Cleanup(roleCleanup)
	spaceClient, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roleClient))
	t.Cleanup(cleanup)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Join Roles"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	inv, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)

	_, err = spaceClient.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)

	got, err := roleClient.GetMemberRoles(joinerCtx, &rolev1.GetMemberRolesRequest{
		SpaceId:   spaceID,
		ProfileId: joinerProfile.String(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.GetRoleList().GetRoles())
	require.Equal(t, permissions.RoleMember, got.GetRoleList().GetRoles()[0].GetName())
}

// TestListMembers_IncludesRoleNames documents SpaceMembership.role_names enrichment.
func TestListMembers_IncludesRoleNames(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	roleClient, roleCleanup := startSharedRoleClient(t)
	t.Cleanup(roleCleanup)
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roleClient))
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Roster"})
	require.NoError(t, err)

	members, err := client.ListMembers(ctx, &spacev1.ListMembersRequest{
		SpaceId: created.GetSpace().GetId(),
		Page:    &commonv1.CursorPageRequest{},
	})
	require.NoError(t, err)
	require.Len(t, members.GetSpaceMemberList().GetMembers(), 1)
	require.Contains(t, members.GetSpaceMemberList().GetMembers()[0].GetRoleNames(), permissions.RoleOwner)
}

// TestCreateInvite_RequiresManageInvitesPermission documents SPACE_MANAGE_INVITES gate (not owner-only).
func TestCreateInvite_RequiresManageInvitesPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	delegateAccount, delegateProfile := uuid.New(), uuid.New()
	delegateCtx := withAccountProfileCtx(context.Background(), delegateAccount, delegateProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	roleClient, roleCleanup := startSharedRoleClient(t)
	t.Cleanup(roleCleanup)
	spaceClient, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roleClient))
	t.Cleanup(cleanup)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Delegates"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	inv, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = spaceClient.JoinByInvite(delegateCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)

	_, err = spaceClient.CreateInvite(delegateCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	roles, err := roleClient.ListRoles(ownerCtx, &rolev1.ListRolesRequest{SpaceId: spaceID})
	require.NoError(t, err)
	var adminRoleID string
	for _, r := range roles.GetRoleList().GetRoles() {
		if r.GetName() == permissions.RoleAdmin {
			adminRoleID = r.GetId()
		}
	}
	require.NotEmpty(t, adminRoleID)
	ownerAssignCtx := metadata.AppendToOutgoingContext(ownerCtx, authctx.HeaderProfileID, created.GetSpace().GetOwnerProfileId())
	_, err = roleClient.AssignRole(ownerAssignCtx, &rolev1.AssignRoleRequest{
		SpaceId:   spaceID,
		ProfileId: delegateProfile.String(),
		RoleId:    adminRoleID,
	})
	require.NoError(t, err)

	_, err = spaceClient.CreateInvite(delegateCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
}
