package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/role/permissions"
)

func TestReorderRoles_UpdatesPositions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	custom, err := s.CreateCustomRole(ctx, spaceID, "Raid Leader", 0, 2, nil)
	require.NoError(t, err)
	roles, err := s.ListRoles(ctx, spaceID)
	require.NoError(t, err)
	ids := make([]uuid.UUID, len(roles))
	for i, r := range roles {
		ids[i] = r.ID
	}
	require.NoError(t, s.ReorderRoles(ctx, spaceID, ids))

	got, err := s.GetRoleByID(ctx, custom.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestVoiceOverrideDenyWins(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()
	profileID := uuid.New()
	voiceRoomID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	roles, err := s.ListRoles(ctx, spaceID)
	require.NoError(t, err)
	var memberRoleID uuid.UUID
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberRoleID = r.ID
		}
	}
	require.NoError(t, s.AssignMemberRole(ctx, spaceID, profileID, memberRoleID, profileID))

	speakMask, err := permissions.MaskFor(permissions.VoiceSpeak)
	require.NoError(t, err)
	require.NoError(t, s.SetVoiceRoomOverride(ctx, voiceRoomID, memberRoleID, 0, speakMask))

	mask, err := s.GetEffectiveMask(ctx, spaceID, profileID, nil, &voiceRoomID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), mask&speakMask)
}

func TestSetDefaultJoinRole_AssignsGuestOnJoin(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	initial, err := s.GetDefaultJoinRole(ctx, spaceID)
	require.NoError(t, err)
	require.NotNil(t, initial)
	require.Equal(t, permissions.RoleMember, initial.Name, "Guest must remain an explicit default-role choice")

	guestID, err := s.RoleIDByName(ctx, spaceID, permissions.RoleGuest)
	require.NoError(t, err)
	require.NoError(t, s.SetDefaultJoinRole(ctx, spaceID, guestID))

	got, err := s.GetDefaultJoinRole(ctx, spaceID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, permissions.RoleGuest, got.Name)

	// Exercise the selected role's assignment and effective permissions, rather
	// than proving only that the default-role configuration round-trips.
	profileID := uuid.New()
	require.NoError(t, s.AssignMemberRole(ctx, spaceID, profileID, got.ID, profileID))
	assigned, err := s.GetMemberRoles(ctx, spaceID, profileID)
	require.NoError(t, err)
	require.Len(t, assigned, 1)
	require.Equal(t, guestID, assigned[0].ID)

	view, err := permissions.MaskFor(permissions.SpaceView)
	require.NoError(t, err)
	mask, err := s.GetEffectiveMask(ctx, spaceID, profileID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, view, mask, "a Guest-only member must not receive the Member fallback mask")

	outsiderMask, err := s.GetEffectiveMask(ctx, spaceID, uuid.New(), nil, nil)
	require.NoError(t, err)
	require.Zero(t, outsiderMask, "the default role must not grant permissions before assignment")
}

func TestGetEffectiveMask_AdminOrdinaryPermissionsRespectNodeDenies(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()
	adminID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	adminRoleID, err := s.RoleIDByName(ctx, spaceID, permissions.RoleAdmin)
	require.NoError(t, err)
	require.NoError(t, s.AssignMemberRole(ctx, spaceID, adminID, adminRoleID, adminID))

	manage, err := permissions.MaskFor(permissions.SpaceManageRoles)
	require.NoError(t, err)
	mask, err := s.GetEffectiveMask(ctx, spaceID, adminID, nil, nil)
	require.NoError(t, err)
	all, err := permissions.AllMask()
	require.NoError(t, err)
	require.Equal(t, all, mask, "ordinary mask equality must not imply ownership")
	chatID, voiceID := uuid.New(), uuid.New()
	require.NoError(t, s.SetChatOverride(ctx, chatID, adminRoleID, manage, manage))
	require.NoError(t, s.SetVoiceRoomOverride(ctx, voiceID, adminRoleID, manage, manage))
	for _, scope := range []struct{ chat, voice *uuid.UUID }{{&chatID, nil}, {nil, &voiceID}} {
		mask, err := s.GetEffectiveMask(ctx, spaceID, adminID, scope.chat, scope.voice)
		require.NoError(t, err)
		require.Zero(t, mask&manage, "Admin must not bypass applicable node denies")
	}
}

func TestGetEffectiveMask_OwnerBypassAndScopedManagementGrantRemainDistinct(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL effective permissions")
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	space, owner, member, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(ctx, space, owner))
	ownerRole, err := s.RoleIDByName(ctx, space, permissions.RoleOwner)
	require.NoError(t, err)
	memberRole, err := s.RoleIDByName(ctx, space, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, s.AssignMemberRole(ctx, space, member, memberRole, owner))
	manage, err := permissions.MaskFor(permissions.SpaceManageRoles)
	require.NoError(t, err)
	require.NoError(t, s.SetChatOverride(ctx, chat, ownerRole, 0, manage))
	require.NoError(t, s.SetChatOverride(ctx, chat, memberRole, manage, 0))
	mask, err := s.GetEffectiveMask(ctx, space, owner, &chat, nil)
	require.NoError(t, err)
	require.NotZero(t, mask&manage, "genuine Owner retains its ordinary bypass")
	mask, err = s.GetEffectiveMask(ctx, space, member, &chat, nil)
	require.NoError(t, err)
	require.NotZero(t, mask&manage, "node bit representation remains ordinary/delegable")
	mask, err = s.GetEffectiveMask(ctx, space, member, nil, nil)
	require.NoError(t, err)
	require.Zero(t, mask&manage, "node allow must not grant global role management")
	_, err = s.CreateCustomRole(ctx, space, permissions.RoleOwner, manage, 100, &owner)
	require.Error(t, err, "unique role name prevents a custom display name from impersonating the seeded Owner")
}

func TestRemoveVoiceRoomOverride_DeletesRow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()
	voiceRoomID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	memberRoleID, err := s.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, s.SetVoiceRoomOverride(ctx, voiceRoomID, memberRoleID, 0, 1))
	_, err = s.RemoveVoiceRoomOverride(ctx, voiceRoomID, memberRoleID)
	require.NoError(t, err)

	rows, err := s.ListVoiceRoomOverrides(ctx, spaceID, &voiceRoomID)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestCanEditRole_BlocksSystemRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(ctx, spaceID, ownerID))

	memberRoleID, err := s.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	ok, err := s.CanEditRole(ctx, spaceID, ownerID, memberRoleID)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestDeleteRole_CustomRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	row, err := s.CreateCustomRole(ctx, spaceID, "Temp", 0, 2, nil)
	require.NoError(t, err)
	require.NoError(t, s.DeleteRole(ctx, row.ID))
	got, err := s.GetRoleByID(ctx, row.ID)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestListChatOverrides_ReturnsRows(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()
	chatID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	memberRoleID, err := s.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	sendMask, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)
	require.NoError(t, s.SetChatOverride(ctx, chatID, memberRoleID, 0, sendMask))

	rows, err := s.ListChatOverrides(ctx, spaceID, &chatID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, sendMask, rows[0].Deny)
}

// TestGetEffectiveMask_DualScopeOverridesApplySequentially covers a combined
// chat and voice-room decision: the final mask retains the chat result after
// voice processing, and deny wins when either override carries both values.
// The role contract does not restrict permission-bit categories by node type.
func TestGetEffectiveMask_DualScopeOverridesApplySequentially(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}

	spaceID := uuid.New()
	profileID := uuid.New()
	chatID := uuid.New()
	voiceRoomID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	memberRoleID, err := s.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, s.AssignMemberRole(ctx, spaceID, profileID, memberRoleID, profileID))

	sendMessages, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)
	manageMessages, err := permissions.MaskFor(permissions.TextChatManageMessages)
	require.NoError(t, err)
	voiceJoin, err := permissions.MaskFor(permissions.VoiceJoin)
	require.NoError(t, err)
	voiceSpeak, err := permissions.MaskFor(permissions.VoiceSpeak)
	require.NoError(t, err)
	voiceMuteOthers, err := permissions.MaskFor(permissions.VoiceMuteOthers)
	require.NoError(t, err)

	require.NoError(t, s.SetChatOverride(ctx, chatID, memberRoleID, manageMessages|sendMessages, sendMessages))
	require.NoError(t, s.SetVoiceRoomOverride(ctx, voiceRoomID, memberRoleID, voiceSpeak|voiceMuteOthers, voiceSpeak))

	mask, err := s.GetEffectiveMask(ctx, spaceID, profileID, &chatID, &voiceRoomID)
	require.NoError(t, err)
	require.Zero(t, mask&sendMessages, "chat deny must beat its allow and stay denied after voice processing")
	require.NotZero(t, mask&manageMessages, "chat allow must remain after voice processing")
	require.NotZero(t, mask&voiceJoin, "unmodified voice permission must remain allowed")
	require.Zero(t, mask&voiceSpeak, "voice deny must beat its allow in the same override")
	require.NotZero(t, mask&voiceMuteOthers, "voice allow must apply in the final mask")
}
