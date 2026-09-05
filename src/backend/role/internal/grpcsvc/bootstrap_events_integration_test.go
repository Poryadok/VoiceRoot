package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/permissions"
)

// TestBootstrapSpaceRoles_PublishesSystemRoleCreatedEventsOnlyOnce ensures a
// retried bootstrap does not re-emit the existing system-role creation events.
func TestBootstrapSpaceRoles_PublishesSystemRoleCreatedEventsOnlyOnce(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()

	events := &recordingRoleEvents{}
	client, stop := startRoleGRPCTestServer(t, s.Pool, func(svc *RoleGRPC) { svc.Events = events })
	defer stop()

	spaceID := uuid.New()
	ownerID := uuid.New()
	request := &rolev1.BootstrapSpaceRolesRequest{
		SpaceId:        spaceID.String(),
		OwnerProfileId: ownerID.String(),
	}

	_, err := client.BootstrapSpaceRoles(context.Background(), request)
	require.NoError(t, err)

	created := events.createdEvents()
	require.Len(t, created, 5, "a new space must publish one role.created event for each system role")
	roleIDs := make(map[string]struct{}, len(created))
	roleNames := make(map[string]struct{}, len(created))
	actualNames := make([]string, 0, len(created))
	for _, event := range created {
		require.Equal(t, spaceID.String(), event.spaceID)
		require.NotEmpty(t, event.roleID)
		require.NotEmpty(t, event.name)
		roleIDs[event.roleID] = struct{}{}
		roleNames[event.name] = struct{}{}
		actualNames = append(actualNames, event.name)
	}
	require.Len(t, roleIDs, 5, "each system role must have a distinct role_id")
	require.Len(t, roleNames, 5, "each system role must have a distinct name")
	require.ElementsMatch(t, []string{
		permissions.RoleOwner,
		permissions.RoleAdmin,
		permissions.RoleModerator,
		permissions.RoleMember,
		permissions.RoleGuest,
	}, actualNames, "a new space must publish the canonical system roles")

	_, err = client.BootstrapSpaceRoles(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, events.createdEvents(), 5, "repeating bootstrap must not publish role.created for existing system roles")
}
