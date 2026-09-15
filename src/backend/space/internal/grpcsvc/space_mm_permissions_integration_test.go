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
	"voice/backend/space/internal/authctx"

	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

func grantSpacePermissionRole(
	t *testing.T,
	roleClient rolev1.RoleServiceClient,
	ownerCtx context.Context,
	spaceID, ownerProfileID, delegateProfileID, name, permission string,
) {
	t.Helper()
	mask, err := permissions.MaskFor(permission)
	require.NoError(t, err)
	ownerRoleCtx := metadata.AppendToOutgoingContext(ownerCtx, authctx.HeaderProfileID, ownerProfileID)
	role, err := roleClient.CreateRole(ownerRoleCtx, &rolev1.CreateRoleRequest{
		SpaceId:         spaceID,
		Name:            name,
		PermissionsMask: mask,
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

func setupSpaceMmPermissionFixture(t *testing.T) (
	spacev1.SpaceServiceClient,
	rolev1.RoleServiceClient,
	string,
	context.Context,
	context.Context,
	context.Context,
	func(),
) {
	t.Helper()
	_, _, ownerCtx := profileFixture(t)
	matchmakingManagerCtx := withAccountProfileCtx(context.Background(), uuid.New(), uuid.New())
	settingsManagerCtx := withAccountProfileCtx(context.Background(), uuid.New(), uuid.New())

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	roleClient, roleCleanup := startSharedRoleClient(t)
	spaceClient, spaceCleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roleClient))

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Matchmaking permissions"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	invite, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = spaceClient.JoinByInvite(matchmakingManagerCtx, &spacev1.JoinByInviteRequest{Code: invite.GetInvite().GetCode()})
	require.NoError(t, err)
	_, err = spaceClient.JoinByInvite(settingsManagerCtx, &spacev1.JoinByInviteRequest{Code: invite.GetInvite().GetCode()})
	require.NoError(t, err)

	matchmakingProfile, _ := profileFixtureFromCtx(matchmakingManagerCtx)
	settingsProfile, _ := profileFixtureFromCtx(settingsManagerCtx)
	grantSpacePermissionRole(t, roleClient, ownerCtx, spaceID, created.GetSpace().GetOwnerProfileId(), matchmakingProfile.String(), "Matchmaking manager", permissions.SpaceManageMatchmaking)
	grantSpacePermissionRole(t, roleClient, ownerCtx, spaceID, created.GetSpace().GetOwnerProfileId(), settingsProfile.String(), "Settings manager", permissions.SpaceManageSettings)

	return spaceClient, roleClient, spaceID, ownerCtx, matchmakingManagerCtx, settingsManagerCtx, func() {
		spaceCleanup()
		roleCleanup()
	}
}

// TestUpdateSpaceMmConfig_AllowsMatchmakingPermission ensures MM configuration
// is delegated by the dedicated SPACE_MANAGE_MATCHMAKING permission.
func TestUpdateSpaceMmConfig_AllowsMatchmakingPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, spaceID, _, matchmakingManagerCtx, _, cleanup := setupSpaceMmPermissionFixture(t)
	t.Cleanup(cleanup)

	updated, err := spaceClient.UpdateSpaceMmConfig(matchmakingManagerCtx, &spacev1.UpdateSpaceMmConfigRequest{
		SpaceId:      spaceID,
		MmConfigJson: `{"game_id":"cs2","region":"eu"}`,
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"game_id":"cs2","region":"eu"}`, updated.GetSpace().GetMmConfigJson())
}

// TestUpdateSpaceMmConfig_RejectsSettingsOnlyPermission ensures ordinary space
// settings authority cannot mutate matchmaking configuration.
func TestUpdateSpaceMmConfig_RejectsSettingsOnlyPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, spaceID, _, _, settingsManagerCtx, cleanup := setupSpaceMmPermissionFixture(t)
	t.Cleanup(cleanup)

	_, err := spaceClient.UpdateSpaceMmConfig(settingsManagerCtx, &spacev1.UpdateSpaceMmConfigRequest{
		SpaceId:      spaceID,
		MmConfigJson: `{"game_id":"dota2","region":"ru"}`,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestUpdateSpace_MmConfigRequiresDedicatedPermission prevents the general
// settings endpoint from bypassing the dedicated matchmaking capability.
func TestUpdateSpace_MmConfigRequiresDedicatedPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, spaceID, _, matchmakingManagerCtx, settingsManagerCtx, cleanup := setupSpaceMmPermissionFixture(t)
	t.Cleanup(cleanup)

	configJSON := `{"game_id":"valorant","region":"na"}`
	updated, err := spaceClient.UpdateSpace(matchmakingManagerCtx, &spacev1.UpdateSpaceRequest{
		SpaceId:      spaceID,
		MmConfigJson: &configJSON,
	})
	require.NoError(t, err)
	require.JSONEq(t, configJSON, updated.GetSpace().GetMmConfigJson())

	configJSON = `{"game_id":"pubg","region":"eu"}`
	_, err = spaceClient.UpdateSpace(settingsManagerCtx, &spacev1.UpdateSpaceRequest{
		SpaceId:      spaceID,
		MmConfigJson: &configJSON,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestUpdateSpace_MmManagerCannotChangeOrdinarySettings ensures a delegated
// matchmaking manager has no implicit ordinary Space settings authority.
func TestUpdateSpace_MmManagerCannotChangeOrdinarySettings(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, spaceID, _, matchmakingManagerCtx, _, cleanup := setupSpaceMmPermissionFixture(t)
	t.Cleanup(cleanup)

	configJSON := `{"game_id":"cs2","region":"eu"}`
	description := "unrelated ordinary setting"
	_, err := spaceClient.UpdateSpace(matchmakingManagerCtx, &spacev1.UpdateSpaceRequest{
		SpaceId:      spaceID,
		MmConfigJson: &configJSON,
		Description:  &description,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
