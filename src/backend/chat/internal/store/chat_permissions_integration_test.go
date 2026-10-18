package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestChatPermissionsPersistAcrossRoleChangesButNotMemberships(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}
	owner, member, other := uuid.New(), uuid.New(), uuid.New()
	row, err := s.CreateGroupChat(ctx, owner, "Permissions", nil)
	require.NoError(t, err)
	_, err = s.AddGroupMembers(ctx, row.ID, []uuid.UUID{member, other})
	require.NoError(t, err)
	p, err := s.GetChatPermissions(ctx, row.ID, member)
	require.NoError(t, err)
	require.True(t, p.CanAddMembers)
	require.False(t, p.CanPostAsChat)
	require.False(t, p.CanManagePermissions)
	require.ErrorIs(t, s.SetChatMemberPostAsChat(ctx, row.ID, member, member, true), ErrRoleChangeForbidden)
	require.NoError(t, s.SetChatMemberPostAsChat(ctx, row.ID, owner, member, true))
	require.NoError(t, s.SetChatMemberPostAsChat(ctx, row.ID, owner, member, true))
	p, err = s.GetChatPermissions(ctx, row.ID, member)
	require.NoError(t, err)
	require.True(t, p.CanPostAsChat)
	require.True(t, p.ExplicitPostAsChat)
	require.NoError(t, s.SetStandaloneGroupMemberRole(ctx, row.ID, owner, member, "admin"))
	require.NoError(t, s.SetStandaloneGroupMemberRole(ctx, row.ID, owner, member, "member"))
	p, err = s.GetChatPermissions(ctx, row.ID, member)
	require.NoError(t, err)
	require.True(t, p.CanPostAsChat)
	// A distinct explicit grant persists during this membership, not across rejoin.
	require.NoError(t, s.TransferGroupOwnership(ctx, row.ID, owner, other))
	p, err = s.GetChatPermissions(ctx, row.ID, owner)
	require.NoError(t, err)
	require.False(t, p.CanPostAsChat)
	// Grow before removal to preserve the existing minimum-members invariant.
	_, err = s.AddGroupMembersAs(ctx, row.ID, other, []uuid.UUID{uuid.New()}, nil)
	require.NoError(t, err)
	require.NoError(t, s.RemoveStandaloneGroupMember(ctx, row.ID, other, member))
	_, err = s.AddGroupMembersAs(ctx, row.ID, other, []uuid.UUID{member}, nil)
	require.NoError(t, err)
	p, err = s.GetChatPermissions(ctx, row.ID, member)
	require.NoError(t, err)
	require.False(t, p.CanPostAsChat)
	require.False(t, p.ExplicitPostAsChat)
}

func TestChatMemberAdditionPolicyDeniesWithoutPartialWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}
	owner, member, target := uuid.New(), uuid.New(), uuid.New()
	row, err := s.CreateChannelChat(ctx, owner, "Permissions", nil)
	require.NoError(t, err)
	_, err = s.AddGroupMembersAs(ctx, row.ID, owner, []uuid.UUID{member}, nil)
	require.NoError(t, err)
	no := false
	_, err = s.UpdateGroupChatWithPermissions(ctx, row.ID, owner, nil, nil, nil, nil, nil, nil, nil, &no)
	require.NoError(t, err)
	_, err = s.AddGroupMembersAs(ctx, row.ID, member, []uuid.UUID{target}, nil)
	require.ErrorIs(t, err, ErrRoleChangeForbidden)
	exists, err := s.IsChatMember(ctx, row.ID, target)
	require.NoError(t, err)
	require.False(t, exists)
	// Manager bypass does not reset the stored toggle.
	_, err = s.AddGroupMembersAs(ctx, row.ID, owner, []uuid.UUID{target}, nil)
	require.NoError(t, err)
	p, err := s.GetChatPermissions(ctx, row.ID, member)
	require.NoError(t, err)
	require.False(t, p.MemberAdditionsEnabled)
	require.False(t, p.CanAddMembers)
	name := "must not change"
	_, err = s.UpdateGroupChatWithPermissions(ctx, row.ID, member, &name, nil, nil, nil, nil, nil, nil, &no)
	require.ErrorIs(t, err, ErrRoleChangeForbidden)
	current, err := s.FindChatByID(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, "Permissions", *current.Name)
	// Standalone permission endpoints cannot create local Space membership.
	spaceChat, err := s.CreateSpaceChannelChat(ctx, owner, uuid.New(), "Space", nil)
	require.NoError(t, err)
	_, err = s.GetChatPermissions(ctx, spaceChat.ID, owner)
	require.ErrorIs(t, err, ErrChatPermissionScope)
	_, err = s.AddGroupMembersAs(ctx, spaceChat.ID, owner, []uuid.UUID{target}, nil)
	require.ErrorIs(t, err, ErrChatPermissionScope)
}
