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
