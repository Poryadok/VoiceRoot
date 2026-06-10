package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/role/permissions"
)

// TestBootstrapSystemRoles_SeedsHierarchy documents PLAN Phase 5 / role-service.md:
// Owner → Admin → Moderator → Member → Guest with descending position priority.
func TestBootstrapSystemRoles_SeedsHierarchy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	spaceID := uuid.New()

	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	roles, err := s.ListRoles(ctx, spaceID)
	require.NoError(t, err)
	require.Len(t, roles, 5)

	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = r.Name
		require.True(t, r.Managed, "system role %q must be managed", r.Name)
	}
	require.Equal(t, []string{
		permissions.RoleOwner,
		permissions.RoleAdmin,
		permissions.RoleModerator,
		permissions.RoleMember,
		permissions.RoleGuest,
	}, names)

	for i := 1; i < len(roles); i++ {
		require.Greater(t, roles[i-1].Position, roles[i].Position,
			"higher role must have greater position: %s vs %s", roles[i-1].Name, roles[i].Name)
	}
}

// TestAssignMemberRole_PersistsMemberRoles documents member_roles UNIQUE(space_id, profile_id, role_id).
func TestAssignMemberRole_PersistsMemberRoles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}

	spaceID := uuid.New()
	ownerID := uuid.New()
	memberID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	roles, err := s.ListRoles(ctx, spaceID)
	require.NoError(t, err)
	var memberRoleID uuid.UUID
	for _, r := range roles {
		if r.Name == permissions.RoleMember {
			memberRoleID = r.ID
		}
	}
	require.NotEqual(t, uuid.Nil, memberRoleID)

	require.NoError(t, s.AssignMemberRole(ctx, spaceID, memberID, memberRoleID, ownerID))

	got, err := s.GetMemberRoles(ctx, spaceID, memberID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, permissions.RoleMember, got[0].Name)
}

// TestGetEffectivePermissions_OwnerBypass documents owner has all permission bits.
func TestGetEffectivePermissions_OwnerBypass(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}

	spaceID := uuid.New()
	ownerID := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	roles, err := s.ListRoles(ctx, spaceID)
	require.NoError(t, err)
	var ownerRoleID uuid.UUID
	for _, r := range roles {
		if r.Name == permissions.RoleOwner {
			ownerRoleID = r.ID
		}
	}
	require.NoError(t, s.AssignMemberRole(ctx, spaceID, ownerID, ownerRoleID, ownerID))

	all, err := permissions.AllMask()
	require.NoError(t, err)
	mask, err := s.GetEffectiveMask(ctx, spaceID, ownerID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, all, mask)
}

// TestGetEffectivePermissions_ChatOverrideDenyWins documents deny in chat_overrides beats allow.
func TestGetEffectivePermissions_ChatOverrideDenyWins(t *testing.T) {
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

	sendMask, err := permissions.MaskFor(permissions.TextChatSendMessages)
	require.NoError(t, err)
	require.NoError(t, s.SetChatOverride(ctx, chatID, memberRoleID, 0, sendMask))

	mask, err := s.GetEffectiveMask(ctx, spaceID, profileID, &chatID, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), mask&sendMask, "deny must remove TEXT_CHAT_SEND_MESSAGES")
}

// TestCanManageRole_Hierarchy documents MEMBER_ASSIGN_ROLES respects role position.
func TestCanManageRole_Hierarchy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}

	spaceID := uuid.New()
	adminProfile := uuid.New()
	require.NoError(t, s.BootstrapSystemRoles(ctx, spaceID))

	roles, err := s.ListRoles(ctx, spaceID)
	require.NoError(t, err)
	byName := map[string]uuid.UUID{}
	for _, r := range roles {
		byName[r.Name] = r.ID
	}
	require.NoError(t, s.AssignMemberRole(ctx, spaceID, adminProfile, byName[permissions.RoleAdmin], adminProfile))

	ok, err := s.CanManageRole(ctx, spaceID, adminProfile, byName[permissions.RoleMember])
	require.NoError(t, err)
	require.True(t, ok, "admin must manage Member role")

	ok, err = s.CanManageRole(ctx, spaceID, adminProfile, byName[permissions.RoleOwner])
	require.NoError(t, err)
	require.False(t, ok, "admin must not manage Owner role")
}
