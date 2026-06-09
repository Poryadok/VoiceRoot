package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestRemoveGroupMember_OwnerForbidden documents PLAN Phase 4 simple roles: owner cannot be kicked.
func TestRemoveGroupMember_OwnerForbidden(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	memberA, memberB := uuid.New(), uuid.New()
	row, err := s.CreateGroupChat(ctx, owner, "Owner protected")
	require.NoError(t, err)
	_, err = s.AddGroupMembers(ctx, row.ID, []uuid.UUID{memberA, memberB})
	require.NoError(t, err)

	err = s.RemoveGroupMember(ctx, row.ID, owner)
	require.Error(t, err, "owner must not be removable via kick")

	role, err := s.GetMemberRole(ctx, row.ID, owner)
	require.NoError(t, err)
	require.Equal(t, "owner", role)
}

// TestAddGroupMembers_AssignsMemberRole documents invited users are persisted as member.
func TestAddGroupMembers_AssignsMemberRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	memberA, memberB := uuid.New(), uuid.New()
	row, err := s.CreateGroupChat(ctx, owner, "Roles")
	require.NoError(t, err)
	_, err = s.AddGroupMembers(ctx, row.ID, []uuid.UUID{memberA, memberB})
	require.NoError(t, err)

	for _, pid := range []uuid.UUID{memberA, memberB} {
		role, err := s.GetMemberRole(ctx, row.ID, pid)
		require.NoError(t, err)
		require.Equal(t, "member", role)
	}
}

// TestLeaveGroupChat_MemberLeaves documents voluntary leave for regular participants.
func TestLeaveGroupChat_MemberLeaves(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	memberA, memberB, leaver := uuid.New(), uuid.New(), uuid.New()
	row, err := s.CreateGroupChat(ctx, owner, "Leave")
	require.NoError(t, err)
	_, err = s.AddGroupMembers(ctx, row.ID, []uuid.UUID{memberA, memberB, leaver})
	require.NoError(t, err)

	require.NoError(t, s.LeaveGroupChat(ctx, row.ID, leaver))
	role, err := s.GetMemberRole(ctx, row.ID, leaver)
	require.NoError(t, err)
	require.Empty(t, role)
}

// TestLeaveGroupChat_OwnerForbidden documents owner must transfer ownership before leaving.
func TestLeaveGroupChat_OwnerForbidden(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	memberA, memberB := uuid.New(), uuid.New()
	row, err := s.CreateGroupChat(ctx, owner, "Owner leave")
	require.NoError(t, err)
	_, err = s.AddGroupMembers(ctx, row.ID, []uuid.UUID{memberA, memberB})
	require.NoError(t, err)

	err = s.LeaveGroupChat(ctx, row.ID, owner)
	require.ErrorIs(t, err, ErrOwnerMustTransfer)

	role, err := s.GetMemberRole(ctx, row.ID, owner)
	require.NoError(t, err)
	require.Equal(t, "owner", role)
}
