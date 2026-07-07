package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/permissions"
)

func TestUpdateRole_CustomRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	created, err := client.CreateRole(ctxWithProfile(ownerID), &rolev1.CreateRoleRequest{
		SpaceId:  spaceID.String(),
		Name:     "Custom",
		Position: 2,
	})
	require.NoError(t, err)

	newName := "Renamed"
	updated, err := client.UpdateRole(ctxWithProfile(ownerID), &rolev1.UpdateRoleRequest{
		RoleId: created.GetRole().GetId(),
		Name:   &newName,
	})
	require.NoError(t, err)
	require.Equal(t, "Renamed", updated.GetRole().GetName())
}

func TestDeleteRole_CustomRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	created, err := client.CreateRole(ctxWithProfile(ownerID), &rolev1.CreateRoleRequest{
		SpaceId:  spaceID.String(),
		Name:     "Disposable",
		Position: 2,
	})
	require.NoError(t, err)

	_, err = client.DeleteRole(ctxWithProfile(ownerID), &rolev1.DeleteRoleRequest{
		RoleId: created.GetRole().GetId(),
	})
	require.NoError(t, err)
}

func TestDeleteRole_SystemRoleDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	var memberID string
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberID = r.ID.String()
		}
	}

	_, err = client.DeleteRole(ctxWithProfile(ownerID), &rolev1.DeleteRoleRequest{RoleId: memberID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestSetChatOverride_PerRoleId(t *testing.T) {
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
	chatID := uuid.New().String()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	var memberRoleID string
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberRoleID = r.ID.String()
			require.NoError(t, s.AssignMemberRole(context.Background(), spaceID, profileID, r.ID, ownerID))
		}
	}

	sendMask, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)
	_, err = client.SetChatOverride(ctxWithProfile(ownerID), &rolev1.SetChatOverrideRequest{
		SpaceId:  spaceID.String(),
		Chat:     &chatv1.ChatRef{Id: chatID},
		RoleId:   memberRoleID,
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

func TestReorderRoles_UpdatesOrder(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	created, err := client.CreateRole(ctxWithProfile(ownerID), &rolev1.CreateRoleRequest{
		SpaceId:  spaceID.String(),
		Name:     "Custom",
		Position: 2,
	})
	require.NoError(t, err)

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	ids := make([]string, len(roles))
	for i, r := range roles {
		ids[i] = r.ID.String()
	}
	// Put custom role first in order (highest position).
	ids[0], ids[1] = created.GetRole().GetId(), ids[0]

	_, err = client.ReorderRoles(ctxWithProfile(ownerID), &rolev1.ReorderRolesRequest{
		SpaceId:        spaceID.String(),
		OrderedRoleIds: ids,
	})
	require.NoError(t, err)
}

func TestSetDefaultJoinRole_GetDefaultJoinRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	guestID, err := s.RoleIDByName(context.Background(), spaceID, permissions.RoleGuest)
	require.NoError(t, err)

	_, err = client.SetDefaultJoinRole(ctxWithProfile(ownerID), &rolev1.SetDefaultJoinRoleRequest{
		SpaceId: spaceID.String(),
		RoleId:  guestID.String(),
	})
	require.NoError(t, err)

	got, err := client.GetDefaultJoinRole(context.Background(), &rolev1.GetDefaultJoinRoleRequest{
		SpaceId: spaceID.String(),
	})
	require.NoError(t, err)
	require.Equal(t, permissions.RoleGuest, got.GetRole().GetName())
}
