package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestAutoUnarchiveDMRecipients_IncomingPeer documents text-chat.md §Архивирование auto-unarchive.
func TestAutoUnarchiveDMRecipients_IncomingPeer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	profA := uuid.New()
	profB := uuid.New()
	dm, _, err := s.EnsureDM(ctx, profA, profB)
	require.NoError(t, err)

	require.NoError(t, s.SetMemberArchived(ctx, dm.ID, profA, true))

	page, err := s.ListChatsPage(ctx, profA, "", 10, "main", nil)
	require.NoError(t, err)
	require.Empty(t, page.Rows)

	require.NoError(t, s.AutoUnarchiveDMRecipients(ctx, dm.ID, profB))

	page, err = s.ListChatsPage(ctx, profA, "", 10, "main", nil)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
	require.Equal(t, dm.ID, page.Rows[0].ID)
}

// TestAutoUnarchiveDMRecipients_OutgoingDoesNotUnarchive documents outgoing from archiver stays archived.
func TestAutoUnarchiveDMRecipients_OutgoingDoesNotUnarchive(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	profA := uuid.New()
	profB := uuid.New()
	dm, _, err := s.EnsureDM(ctx, profA, profB)
	require.NoError(t, err)

	require.NoError(t, s.SetMemberArchived(ctx, dm.ID, profA, true))
	require.NoError(t, s.AutoUnarchiveDMRecipients(ctx, dm.ID, profA))

	page, err := s.ListChatsPage(ctx, profA, "", 10, "main", nil)
	require.NoError(t, err)
	require.Empty(t, page.Rows)
}

// TestAutoUnarchiveDMRecipients_SkipsGroupChat documents group/channel do not auto-unarchive.
func TestAutoUnarchiveDMRecipients_SkipsGroupChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	chatID := seedMembershipChannel(t, ctx, s, owner, "Group-ish channel")
	require.NoError(t, s.SetMemberArchived(ctx, chatID, owner, true))

	require.NoError(t, s.AutoUnarchiveDMRecipients(ctx, chatID, uuid.New()))

	page, err := s.ListChatsPage(ctx, owner, "", 10, "main", nil)
	require.NoError(t, err)
	require.Empty(t, page.Rows)
}
