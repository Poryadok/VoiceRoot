package store

import (
	"context"
	"testing"
	"time"

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

func TestChatPermissionMutationsRecheckAfterConcurrentAuthorityChange(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}
	owner, admin, member, target := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	row, err := s.CreateChannelChat(ctx, owner, "Concurrent", nil)
	require.NoError(t, err)
	_, err = s.AddGroupMembersAs(ctx, row.ID, owner, []uuid.UUID{admin, member}, nil)
	require.NoError(t, err)
	waitBlocked := func(t *testing.T) {
		t.Helper()
		require.Eventually(t, func() bool {
			var waiting bool
			err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid<>pg_backend_pid()
    AND wait_event_type='Lock' AND query LIKE '%chats%FOR UPDATE%')`).Scan(&waiting)
			return err == nil && waiting
		}, 5*time.Second, 10*time.Millisecond, "the real mutation must wait on the chat lock")
	}
	for _, operation := range []string{"grant", "toggle"} {
		t.Run(operation+" after manager demotion", func(t *testing.T) {
			_, err := pool.Exec(ctx, "UPDATE chat_members SET role='admin' WHERE chat_id=$1 AND profile_id=$2", row.ID, admin)
			require.NoError(t, err)
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(ctx) }()
			_, err = tx.Exec(ctx, "SELECT id FROM chats WHERE id=$1 FOR UPDATE", row.ID)
			require.NoError(t, err)
			result := make(chan error, 1)
			go func() {
				if operation == "grant" {
					result <- s.SetChatMemberPostAsChat(ctx, row.ID, admin, member, true)
					return
				}
				disabled := false
				_, err := s.UpdateGroupChatWithPermissions(ctx, row.ID, admin, nil, nil, nil, nil, nil, nil, nil, &disabled)
				result <- err
			}()
			waitBlocked(t)
			_, err = tx.Exec(ctx, "UPDATE chat_members SET role='member' WHERE chat_id=$1 AND profile_id=$2", row.ID, admin)
			require.NoError(t, err)
			require.NoError(t, tx.Commit(ctx))
			select {
			case err := <-result:
				require.ErrorIs(t, err, ErrRoleChangeForbidden)
			case <-time.After(5 * time.Second):
				t.Fatal("mutation did not finish")
			}
			p, err := s.GetChatPermissions(ctx, row.ID, member)
			require.NoError(t, err)
			require.False(t, p.ExplicitPostAsChat)
			require.True(t, p.MemberAdditionsEnabled)
		})
	}
	t.Run("add after caller removal", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, "SELECT id FROM chats WHERE id=$1 FOR UPDATE", row.ID)
		require.NoError(t, err)
		result := make(chan error, 1)
		go func() { _, err := s.AddGroupMembersAs(ctx, row.ID, member, []uuid.UUID{target}, nil); result <- err }()
		waitBlocked(t)
		_, err = tx.Exec(ctx, "DELETE FROM chat_members WHERE chat_id=$1 AND profile_id=$2", row.ID, member)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
		select {
		case err := <-result:
			require.ErrorIs(t, err, ErrNotGroupMember)
		case <-time.After(5 * time.Second):
			t.Fatal("add did not finish")
		}
		exists, err := s.IsChatMember(ctx, row.ID, target)
		require.NoError(t, err)
		require.False(t, exists)
	})
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
