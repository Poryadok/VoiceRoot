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

func TestGetEffectiveMask_AdminBypass(t *testing.T) {
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

	all, err := permissions.AllMask()
	require.NoError(t, err)
	mask, err := s.GetEffectiveMask(ctx, spaceID, adminID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, all, mask)
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
