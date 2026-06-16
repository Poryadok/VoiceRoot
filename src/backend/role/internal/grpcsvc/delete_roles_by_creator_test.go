package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/permissions"
)

// TestDeleteRolesCreatedByProfile_deletesOnlyNonSystemRoles documents uninstall cleanup:
// only non-system roles where created_by_profile_id matches are removed.
func TestDeleteRolesCreatedByProfile_deletesOnlyNonSystemRoles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	botProfileID := uuid.New()
	otherProfileID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, ownerID))

	sendMask, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)

	botRole, err := client.CreateRole(ctxWithProfile(ownerID), &rolev1.CreateRoleRequest{
		SpaceId:         spaceID.String(),
		Name:            "BotRaidLeader",
		PermissionsMask: sendMask,
		Position:        2,
	})
	require.NoError(t, err)
	require.NotEmpty(t, botRole.GetRole().GetId())

	otherRole, err := client.CreateRole(ctxWithProfile(ownerID), &rolev1.CreateRoleRequest{
		SpaceId:         spaceID.String(),
		Name:            "HumanCustom",
		PermissionsMask: sendMask,
		Position:        3,
	})
	require.NoError(t, err)

	// Mark bot-created role once created_by_profile_id column lands (BOT-B).
	_, err = s.Pool.Exec(context.Background(), `
UPDATE roles SET created_by_profile_id = $1 WHERE id = $2
`, botProfileID, uuid.MustParse(botRole.GetRole().GetId()))
	if err != nil {
		t.Fatalf("roles.created_by_profile_id column required for BOT-B uninstall cleanup: %v", err)
	}
	_, err = s.Pool.Exec(context.Background(), `
UPDATE roles SET created_by_profile_id = $1 WHERE id = $2
`, otherProfileID, uuid.MustParse(otherRole.GetRole().GetId()))
	require.NoError(t, err)

	_, err = client.DeleteRolesCreatedByProfile(context.Background(), &rolev1.DeleteRolesCreatedByProfileRequest{
		SpaceId:            spaceID.String(),
		CreatedByProfileId: botProfileID.String(),
	})
	require.NoError(t, err)

	after, err := client.ListRoles(context.Background(), &rolev1.ListRolesRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	roles := after.GetRoleList().GetRoles()
	require.Len(t, roles, 6, "system roles + human custom role must remain")

	var names []string
	for _, r := range roles {
		names = append(names, r.GetName())
		require.NotEqual(t, "BotRaidLeader", r.GetName(),
			"bot-created custom role must be deleted")
	}
	require.Contains(t, names, permissions.RoleOwner)
	require.Contains(t, names, "HumanCustom")
}
