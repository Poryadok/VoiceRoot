package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/internal/authctx"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"
)

func ctxWithProfile(profileID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), authctx.HeaderProfileID, profileID.String())
}

func startRoleStoreTest(t *testing.T) (*store.RoleStore, func()) {
	t.Helper()
	ctx := context.Background()
	pool := store.StartRoleDBForStoreTest(t, ctx)
	store.ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	return &store.RoleStore{Pool: pool}, func() { pool.Close() }
}

// TestListRoles_AfterBootstrap documents RoleService.ListRoles returns system hierarchy.
func TestListRoles_AfterBootstrap(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(context.Background(), spaceID))

	resp, err := client.ListRoles(context.Background(), &rolev1.ListRolesRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	roles := resp.GetRoleList().GetRoles()
	require.Len(t, roles, 5)
	require.Equal(t, permissions.RoleOwner, roles[0].GetName())
}

// TestCreateRole_CustomRole documents custom roles with permissions_mask.
func TestCreateRole_CustomRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New().String()
	require.NoError(t, s.BootstrapSystemRoles(context.Background(), uuid.MustParse(spaceID)))

	sendMask, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)

	resp, err := client.CreateRole(context.Background(), &rolev1.CreateRoleRequest{
		SpaceId:         spaceID,
		Name:            "Raid Leader",
		PermissionsMask: sendMask,
		Position:        2,
	})
	require.NoError(t, err)
	require.Equal(t, "Raid Leader", resp.GetRole().GetName())
	require.Equal(t, sendMask, resp.GetRole().GetPermissionsMask())
}

// TestAssignRole_GetMemberRoles documents role assignment RPCs.
func TestAssignRole_GetMemberRoles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	profileID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	var memberRoleID string
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberRoleID = r.ID.String()
		}
	}

	_, err = client.AssignRole(ctxWithProfile(ownerID), &rolev1.AssignRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: profileID.String(),
		RoleId:    memberRoleID,
	})
	require.NoError(t, err)

	got, err := client.GetMemberRoles(context.Background(), &rolev1.GetMemberRolesRequest{
		SpaceId:   spaceID.String(),
		ProfileId: profileID.String(),
	})
	require.NoError(t, err)
	require.Len(t, got.GetRoleList().GetRoles(), 1)
	require.Equal(t, permissions.RoleMember, got.GetRoleList().GetRoles()[0].GetName())
}

// TestCheckPermission_SpaceScope documents CheckPermission for SPACE_MANAGE_INVITES.
func TestCheckPermission_SpaceScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	memberID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(context.Background(), spaceID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			require.NoError(t, s.AssignMemberRole(context.Background(), spaceID, memberID, r.ID, memberID))
		}
	}

	resp, err := client.CheckPermission(context.Background(), &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      memberID.String(),
		PermissionName: permissions.SpaceManageInvites,
	})
	require.NoError(t, err)
	require.False(t, resp.GetAllowed())
}

// TestSetChatOverride_CheckPermission documents chat-scoped deny override.
func TestSetChatOverride_CheckPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	profileID := uuid.New()
	chatID := uuid.New().String()
	require.NoError(t, s.BootstrapSystemRoles(context.Background(), spaceID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			require.NoError(t, s.AssignMemberRole(context.Background(), spaceID, profileID, r.ID, profileID))
		}
	}

	sendMask, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)
	_, err = client.SetChatOverride(context.Background(), &rolev1.SetChatOverrideRequest{
		SpaceId:  spaceID.String(),
		Chat:     &chatv1.ChatRef{Id: chatID},
		DenyMask: sendMask,
	})
	require.NoError(t, err)

	resp, err := client.CheckPermission(context.Background(), &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      profileID.String(),
		PermissionName: permissions.TextChatSendMessages,
		Chat:           &chatv1.ChatRef{Id: chatID},
	})
	require.NoError(t, err)
	require.False(t, resp.GetAllowed())
}

// TestAssignRole_HierarchyDenied documents admin cannot assign Owner role.
func TestAssignRole_HierarchyDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	adminID := uuid.New()
	targetID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(context.Background(), spaceID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	byName := map[string]uuid.UUID{}
	for _, r := range roles {
		byName[r.Name] = r.ID
	}
	require.NoError(t, s.AssignMemberRole(context.Background(), spaceID, adminID, byName[permissions.RoleAdmin], adminID))

	_, err = client.AssignRole(ctxWithProfile(adminID), &rolev1.AssignRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: targetID.String(),
		RoleId:    byName[permissions.RoleOwner].String(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
