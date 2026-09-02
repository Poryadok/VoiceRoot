package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	commonv1 "voice.app/voice/common/v1"

	chatv1 "voice.app/voice/chat/v1"
)

type mapDeletedAccounts map[uuid.UUID]struct{}

func (m mapDeletedAccounts) DeletedAmong(_ context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	out := make(map[uuid.UUID]struct{})
	for _, id := range accountIDs {
		if _, ok := m[id]; ok {
			out[id] = struct{}{}
		}
	}
	return out, nil
}

func WithAccountDeletedChecker(c AccountDeletedChecker) chatServerOption {
	return func(s *ChatGRPC) { s.DeletedAccounts = c }
}

// TestListChats_HidesDMWhenPeerAccountDeleted documents auth-and-contacts.md: DM with deleted peer is omitted from ListChats.
func TestListChats_HidesDMWhenPeerAccountDeleted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	accC := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB, profC: accC}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithAccountDeletedChecker(mapDeletedAccounts{accB: {}}))
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	_, err = client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profC.String()})
	require.NoError(t, err)

	list, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	items := list.GetChatList().GetItems()
	require.Len(t, items, 1, "deleted-peer DM must be hidden; active DM remains")
	require.Equal(t, profC.String(), items[0].GetDmPeerProfileId())
}
