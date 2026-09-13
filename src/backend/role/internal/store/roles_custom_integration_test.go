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

	guestID, err := s.RoleIDByName(ctx, spaceID, permissions.RoleGuest)
	require.NoError(t, err)
	require.NoError(t, s.SetDefaultJoinRole(ctx, spaceID, guestID))

	got, err := s.GetDefaultJoinRole(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, permissions.RoleGuest, got.Name)
}

func TestGetEffectiveMask_AdminNotOwnerExclusive(t *testing.T) {
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

	ownerExclusive, err := permissions.OwnerExclusiveMask()
	require.NoError(t, err)
	mask, err := s.GetEffectiveMask(ctx, spaceID, adminID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), mask&ownerExclusive)
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
	require.NoError(t, s.RemoveVoiceRoomOverride(ctx, voiceRoomID, memberRoleID))

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
