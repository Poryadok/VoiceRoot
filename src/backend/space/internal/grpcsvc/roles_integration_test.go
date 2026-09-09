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

func grantManageInvitesRole(t *testing.T, roleClient rolev1.RoleServiceClient, ownerCtx context.Context, spaceID, ownerProfileID, delegateProfileID string) {
	t.Helper()
	inviteMask, err := permissions.MaskFor(permissions.SpaceManageInvites)
	require.NoError(t, err)
	ownerRoleCtx := metadata.AppendToOutgoingContext(ownerCtx, authctx.HeaderProfileID, ownerProfileID)
	role, err := roleClient.CreateRole(ownerRoleCtx, &rolev1.CreateRoleRequest{
		SpaceId:         spaceID,
		Name:            "Invite manager",
		PermissionsMask: inviteMask,
		Position:        2,
	})
	require.NoError(t, err)
	_, err = roleClient.AssignRole(ownerRoleCtx, &rolev1.AssignRoleRequest{
		SpaceId:   spaceID,
		ProfileId: delegateProfileID,
		RoleId:    role.GetRole().GetId(),
	})
	require.NoError(t, err)
}

// TestListInvites_RequiresManageInvitesPermission documents the canonical
// invite-management contract: a joined member needs SPACE_MANAGE_INVITES, and
// a member granted that permission may list the space's invites without being
// its owner.
func TestListInvites_RequiresManageInvitesPermission(t *testing.T) {
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

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Invite list delegates"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	admission, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = spaceClient.JoinByInvite(delegateCtx, &spacev1.JoinByInviteRequest{Code: admission.GetInvite().GetCode()})
	require.NoError(t, err)

	_, err = spaceClient.ListInvites(delegateCtx, &spacev1.ListInvitesRequest{SpaceId: spaceID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	grantManageInvitesRole(t, roleClient, ownerCtx, spaceID, created.GetSpace().GetOwnerProfileId(), delegateProfile.String())

	list, err := spaceClient.ListInvites(delegateCtx, &spacev1.ListInvitesRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Len(t, list.GetInviteList().GetInvites(), 2)
}

// TestRevokeInvite_RequiresManageInvitesPermission documents that a joined
// member with SPACE_MANAGE_INVITES may revoke an invite. A member without the
// permission remains denied, and a successful revoke removes the invite from
// the active list and rejects subsequent redemption.
func TestRevokeInvite_RequiresManageInvitesPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	delegateAccount, delegateProfile := uuid.New(), uuid.New()
	delegateCtx := withAccountProfileCtx(context.Background(), delegateAccount, delegateProfile)
	joinerCtx := withAccountProfileCtx(context.Background(), uuid.New(), uuid.New())

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	roleClient, roleCleanup := startSharedRoleClient(t)
	t.Cleanup(roleCleanup)
	spaceClient, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roleClient))
	t.Cleanup(cleanup)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Invite revoke delegates"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	admission, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	target, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = spaceClient.JoinByInvite(delegateCtx, &spacev1.JoinByInviteRequest{Code: admission.GetInvite().GetCode()})
	require.NoError(t, err)

	_, err = spaceClient.RevokeInvite(delegateCtx, &spacev1.RevokeInviteRequest{InviteId: target.GetInvite().GetId()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	beforeGrant, err := spaceClient.ListInvites(ownerCtx, &spacev1.ListInvitesRequest{SpaceId: spaceID})
	require.NoError(t, err)
	var unrevoked *spacev1.Invite
	for _, invite := range beforeGrant.GetInviteList().GetInvites() {
		if invite.GetId() == target.GetInvite().GetId() {
			unrevoked = invite
		}
	}
	require.NotNil(t, unrevoked)
	require.Nil(t, unrevoked.GetRevokedAt())

	grantManageInvitesRole(t, roleClient, ownerCtx, spaceID, created.GetSpace().GetOwnerProfileId(), delegateProfile.String())

	_, err = spaceClient.RevokeInvite(delegateCtx, &spacev1.RevokeInviteRequest{InviteId: target.GetInvite().GetId()})
	require.NoError(t, err)

	list, err := spaceClient.ListInvites(ownerCtx, &spacev1.ListInvitesRequest{SpaceId: spaceID})
	require.NoError(t, err)
	var revoked bool
	for _, invite := range list.GetInviteList().GetInvites() {
		if invite.GetId() == target.GetInvite().GetId() {
			revoked = true
		}
	}
	require.False(t, revoked)

	_, err = spaceClient.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: target.GetInvite().GetCode()})
	require.Equal(t, codes.NotFound, status.Code(err))
}
