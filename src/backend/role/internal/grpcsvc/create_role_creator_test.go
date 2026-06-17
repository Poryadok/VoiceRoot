package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/permissions"
)

// TestCreateRole_setsCreatedByProfileID documents that custom roles record the creator profile
// so bot uninstall can delete roles via DeleteRolesCreatedByProfile (BOT-C).
func TestCreateRole_setsCreatedByProfileID(t *testing.T) {
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

	sendMask, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)

	resp, err := client.CreateRole(ctxWithProfile(ownerID), &rolev1.CreateRoleRequest{
		SpaceId:         spaceID.String(),
		Name:            "BotCreatedRole",
		PermissionsMask: sendMask,
		Position:        2,
	})
	require.NoError(t, err)
	require.Equal(t, ownerID.String(), resp.GetRole().GetCreatedByProfileId(),
		"CreateRole must persist created_by_profile_id from actor profile (BOT-C)")
}
