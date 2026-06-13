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

func TestGetVoiceRoomOverrides_SetAndList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	voiceRoomID := uuid.New().String()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	var memberRoleID string
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberRoleID = r.ID.String()
		}
	}

	speakMask, err := permissions.MaskFor(permissions.VoiceSpeak)
	require.NoError(t, err)
	_, err = client.SetVoiceRoomOverride(ctxWithProfile(ownerID), &rolev1.SetVoiceRoomOverrideRequest{
		SpaceId:     spaceID.String(),
		VoiceRoomId: voiceRoomID,
		RoleId:      memberRoleID,
		DenyMask:    speakMask,
	})
	require.NoError(t, err)

	list, err := client.GetVoiceRoomOverrides(context.Background(), &rolev1.GetVoiceRoomOverridesRequest{
		SpaceId:     spaceID.String(),
		VoiceRoomId: &voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, list.GetOverrideList().GetOverrides(), 1)
}

func TestRemoveChatOverride_ClearsRow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	chatID := uuid.New().String()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	roles, err := s.ListRoles(context.Background(), spaceID)
	require.NoError(t, err)
	var memberRoleID string
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberRoleID = r.ID.String()
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

	_, err = client.RemoveChatOverride(ctxWithProfile(ownerID), &rolev1.RemoveChatOverrideRequest{
		SpaceId: spaceID.String(),
		Chat:    &chatv1.ChatRef{Id: chatID},
		RoleId:  memberRoleID,
	})
	require.NoError(t, err)

	list, err := client.GetChatOverrides(context.Background(), &rolev1.GetChatOverridesRequest{
		SpaceId:    spaceID.String(),
		FilterChat: &chatv1.ChatRef{Id: chatID},
	})
	require.NoError(t, err)
	require.Empty(t, list.GetOverrideList().GetOverrides())
}

func TestCreateRole_RequiresManageRolesPermission(t *testing.T) {
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

	_, err = client.CreateRole(ctxWithProfile(memberID), &rolev1.CreateRoleRequest{
		SpaceId: spaceID.String(),
		Name:    "Nope",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetEffectivePermissions_ReturnsMask(t *testing.T) {
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

	resp, err := client.GetEffectivePermissions(context.Background(), &rolev1.GetEffectivePermissionsRequest{
		SpaceId:   spaceID.String(),
		ProfileId: ownerID.String(),
	})
	require.NoError(t, err)
	require.NotZero(t, resp.GetPermissionSet().GetEffectiveMask())
	require.NotEmpty(t, resp.GetPermissionSet().GetPermissionNames())
}
