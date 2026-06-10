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

var moderationProfileAccounts mapProfileAccounts

func startSpaceModerationFixture(t *testing.T) (
	spaceClient spacev1.SpaceServiceClient,
	roleClient rolev1.RoleServiceClient,
	ownerProfile uuid.UUID,
	ownerCtx context.Context,
	cleanup func(),
) {
	t.Helper()
	ownerProfile, ownerAccount, ownerCtx := profileFixture(t)
	moderationProfileAccounts = mapProfileAccounts{ownerProfile: ownerAccount}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	roleClient, roleCleanup := startSharedRoleClient(t)
	spaceClient, spaceCleanup := startSpaceGRPCTestServer(t, pool,
		withRoleClient(roleClient),
		withProfileAccounts(moderationProfileAccounts),
	)
	cleanup = func() {
		spaceCleanup()
		roleCleanup()
	}
	return spaceClient, roleClient, ownerProfile, ownerCtx, cleanup
}

func moderationCtx(accountID, profileID uuid.UUID) context.Context {
	moderationProfileAccounts[profileID] = accountID
	return withAccountProfileCtx(context.Background(), accountID, profileID)
}

func joinSpaceViaInvite(
	t *testing.T,
	spaceClient spacev1.SpaceServiceClient,
	ownerCtx, joinerCtx context.Context,
	spaceID string,
) {
	t.Helper()
	inv, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = spaceClient.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)
}

func assignSpaceRoleByName(
	t *testing.T,
	roleClient rolev1.RoleServiceClient,
	ownerCtx context.Context,
	ownerProfileID uuid.UUID,
	spaceID string,
	profileID uuid.UUID,
	roleName string,
) {
	t.Helper()
	list, err := roleClient.ListRoles(ownerCtx, &rolev1.ListRolesRequest{SpaceId: spaceID})
	require.NoError(t, err)
	var roleID string
	for _, r := range list.GetRoleList().GetRoles() {
		if r.GetName() == roleName {
			roleID = r.GetId()
		}
	}
	require.NotEmpty(t, roleID, "role %q not found", roleName)
	ownerAssignCtx := metadata.AppendToOutgoingContext(ownerCtx, authctx.HeaderProfileID, ownerProfileID.String())
	_, err = roleClient.AssignRole(ownerAssignCtx, &rolev1.AssignRoleRequest{
		SpaceId:   spaceID,
		ProfileId: profileID.String(),
		RoleId:    roleID,
	})
	require.NoError(t, err)
}

func profileFixtureFromCtx(ctx context.Context) (profileID, accountID uuid.UUID) {
	md, _ := metadata.FromOutgoingContext(ctx)
	for _, v := range md.Get(authctx.HeaderProfileID) {
		if id, err := uuid.Parse(v); err == nil {
			profileID = id
		}
	}
	for _, v := range md.Get(authctx.HeaderUserID) {
		if id, err := uuid.Parse(v); err == nil {
			accountID = id
		}
	}
	return profileID, accountID
}

func accountFromCtx(ctx context.Context) uuid.UUID {
	_, accountID := profileFixtureFromCtx(ctx)
	return accountID
}

// TestSpaceModeration_KickMember_ModeratorCanKick documents PLAN Phase 5: moderator with MEMBER_KICK removes member.
func TestSpaceModeration_KickMember_ModeratorCanKick(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, roleClient, ownerProfile, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	modAccount, modProfile := uuid.New(), uuid.New()
	modCtx := moderationCtx(modAccount, modProfile)
	targetAccount, targetProfile := uuid.New(), uuid.New()
	targetCtx := moderationCtx(targetAccount, targetProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Kick mod"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, modCtx, spaceID)
	joinSpaceViaInvite(t, spaceClient, ownerCtx, targetCtx, spaceID)
	assignSpaceRoleByName(t, roleClient, ownerCtx, ownerProfile, spaceID, modProfile, permissions.RoleModerator)

	_, err = spaceClient.KickMember(modCtx, &spacev1.KickMemberRequest{
		SpaceId:   spaceID,
		ProfileId: targetProfile.String(),
	})
	require.NoError(t, err)

	members, err := spaceClient.ListMembers(ownerCtx, &spacev1.ListMembersRequest{SpaceId: spaceID})
	require.NoError(t, err)
	for _, m := range members.GetSpaceMemberList().GetMembers() {
		require.NotEqual(t, targetProfile.String(), m.GetProfileId())
	}
}

// TestSpaceModeration_KickMember_MemberForbidden documents regular members cannot kick.
func TestSpaceModeration_KickMember_MemberForbidden(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, _, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	joinerAccount, joinerProfile := uuid.New(), uuid.New()
	joinerCtx := moderationCtx(joinerAccount, joinerProfile)
	victimAccount, victimProfile := uuid.New(), uuid.New()
	victimCtx := moderationCtx(victimAccount, victimProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Kick deny"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, joinerCtx, spaceID)
	joinSpaceViaInvite(t, spaceClient, ownerCtx, victimCtx, spaceID)

	_, err = spaceClient.KickMember(joinerCtx, &spacev1.KickMemberRequest{
		SpaceId:   spaceID,
		ProfileId: victimProfile.String(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestSpaceModeration_KickMember_CannotKickOwner documents owner is not kickable.
func TestSpaceModeration_KickMember_CannotKickOwner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, roleClient, ownerProfile, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	modAccount, modProfile := uuid.New(), uuid.New()
	modCtx := moderationCtx(modAccount, modProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Kick owner"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, modCtx, spaceID)
	assignSpaceRoleByName(t, roleClient, ownerCtx, ownerProfile, spaceID, modProfile, permissions.RoleModerator)

	_, err = spaceClient.KickMember(modCtx, &spacev1.KickMemberRequest{
		SpaceId:   spaceID,
		ProfileId: ownerProfile.String(),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestSpaceModeration_KickMember_NotFound documents kick of non-member returns NotFound.
func TestSpaceModeration_KickMember_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, _, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Kick missing"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	_, err = spaceClient.KickMember(ownerCtx, &spacev1.KickMemberRequest{
		SpaceId:   spaceID,
		ProfileId: uuid.New().String(),
	})
	require.Equal(t, codes.NotFound, status.Code(err))
}

// TestSpaceModeration_BanMember_RemovesMember documents ban evicts member from roster.
func TestSpaceModeration_BanMember_RemovesMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, ownerProfile, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	targetAccount, targetProfile := uuid.New(), uuid.New()
	targetCtx := moderationCtx(targetAccount, targetProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Ban evict"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, targetCtx, spaceID)

	_, err = spaceClient.BanMember(ownerCtx, &spacev1.BanMemberRequest{
		SpaceId:   spaceID,
		AccountId: targetAccount.String(),
		Reason:    strPtr("spam"),
	})
	require.NoError(t, err)

	members, err := spaceClient.ListMembers(ownerCtx, &spacev1.ListMembersRequest{SpaceId: spaceID})
	require.NoError(t, err)
	for _, m := range members.GetSpaceMemberList().GetMembers() {
		require.NotEqual(t, targetProfile.String(), m.GetProfileId())
	}
	_ = ownerProfile
}

// TestSpaceModeration_BanMember_BannedCannotJoin documents banned account cannot rejoin via invite.
func TestSpaceModeration_BanMember_BannedCannotJoin(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, _, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	targetAccount, targetProfile := uuid.New(), uuid.New()
	targetCtx := moderationCtx(targetAccount, targetProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Ban rejoin"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, targetCtx, spaceID)

	_, err = spaceClient.BanMember(ownerCtx, &spacev1.BanMemberRequest{
		SpaceId:   spaceID,
		AccountId: targetAccount.String(),
	})
	require.NoError(t, err)

	inv, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = spaceClient.JoinByInvite(targetCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.Error(t, err)
	require.True(t, status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.FailedPrecondition,
		"banned user must not join, got %v", err)
	_ = targetProfile
}

// TestSpaceModeration_UnbanMember_RestoresJoin documents unban allows invite join again.
func TestSpaceModeration_UnbanMember_RestoresJoin(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, _, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	targetAccount, targetProfile := uuid.New(), uuid.New()
	targetCtx := moderationCtx(targetAccount, targetProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Unban"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, targetCtx, spaceID)

	_, err = spaceClient.BanMember(ownerCtx, &spacev1.BanMemberRequest{
		SpaceId:   spaceID,
		AccountId: targetAccount.String(),
	})
	require.NoError(t, err)
	_, err = spaceClient.UnbanMember(ownerCtx, &spacev1.UnbanMemberRequest{
		SpaceId:   spaceID,
		AccountId: targetAccount.String(),
	})
	require.NoError(t, err)

	inv, err := spaceClient.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = spaceClient.JoinByInvite(targetCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)
	_ = targetProfile
}

// TestSpaceModeration_ListBans_IncludesBan documents ListBans returns active bans.
func TestSpaceModeration_ListBans_IncludesBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, _, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	targetAccount, targetProfile := uuid.New(), uuid.New()
	targetCtx := moderationCtx(targetAccount, targetProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "List bans"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, targetCtx, spaceID)

	reason := "abuse"
	_, err = spaceClient.BanMember(ownerCtx, &spacev1.BanMemberRequest{
		SpaceId:   spaceID,
		AccountId: targetAccount.String(),
		Reason:    &reason,
	})
	require.NoError(t, err)

	bans, err := spaceClient.ListBans(ownerCtx, &spacev1.ListBansRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.NotEmpty(t, bans.GetBanList().GetBans())
	found := false
	for _, b := range bans.GetBanList().GetBans() {
		if b.GetAccountId() == targetAccount.String() {
			found = true
			require.Equal(t, reason, b.GetReason())
		}
	}
	require.True(t, found, "ban list must include banned account")
}

// TestSpaceModeration_BanMember_CannotBanOwner documents owner cannot be banned.
func TestSpaceModeration_BanMember_CannotBanOwner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, ownerProfile, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Ban owner"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	ownerAccount := accountFromCtx(ownerCtx)

	_, err = spaceClient.BanMember(ownerCtx, &spacev1.BanMemberRequest{
		SpaceId:   spaceID,
		AccountId: ownerAccount.String(),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_ = ownerProfile
}

// TestSpaceModeration_TimeoutMember_ModeratorCanSet documents MODERATION_TIMEOUT_MEMBERS gate.
func TestSpaceModeration_TimeoutMember_ModeratorCanSet(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, roleClient, ownerProfile, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	modAccount, modProfile := uuid.New(), uuid.New()
	modCtx := moderationCtx(modAccount, modProfile)
	targetAccount, targetProfile := uuid.New(), uuid.New()
	targetCtx := moderationCtx(targetAccount, targetProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Timeout"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, modCtx, spaceID)
	joinSpaceViaInvite(t, spaceClient, ownerCtx, targetCtx, spaceID)
	assignSpaceRoleByName(t, roleClient, ownerCtx, ownerProfile, spaceID, modProfile, permissions.RoleModerator)

	_, err = spaceClient.TimeoutMember(modCtx, &spacev1.TimeoutMemberRequest{
		SpaceId:         spaceID,
		ProfileId:       targetProfile.String(),
		DurationSeconds: 600,
	})
	require.NoError(t, err)
}

// TestSpaceModeration_RemoveMemberTimeout_ClearsTimeout documents timeout removal.
func TestSpaceModeration_RemoveMemberTimeout_ClearsTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceClient, _, _, ownerCtx, cleanup := startSpaceModerationFixture(t)
	t.Cleanup(cleanup)

	targetAccount, targetProfile := uuid.New(), uuid.New()
	targetCtx := moderationCtx(targetAccount, targetProfile)

	created, err := spaceClient.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Untimeout"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	joinSpaceViaInvite(t, spaceClient, ownerCtx, targetCtx, spaceID)

	_, err = spaceClient.TimeoutMember(ownerCtx, &spacev1.TimeoutMemberRequest{
		SpaceId:         spaceID,
		ProfileId:       targetProfile.String(),
		DurationSeconds: 300,
	})
	require.NoError(t, err)
	_, err = spaceClient.RemoveMemberTimeout(ownerCtx, &spacev1.RemoveMemberTimeoutRequest{
		SpaceId:   spaceID,
		ProfileId: targetProfile.String(),
	})
	require.NoError(t, err)
}

func strPtr(s string) *string { return &s }
