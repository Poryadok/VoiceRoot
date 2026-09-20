package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

// TestQuickAccess_ListAddRemoveReorder documents chat-service.md § Quick Access RPCs.
func TestQuickAccess_ListAddRemoveReorder(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	acc := uuid.New()
	prof := uuid.New()
	peers := make([]uuid.UUID, 16)
	for i := range peers {
		peers[i] = uuid.New()
	}
	profiles := mapProfileAccounts{prof: acc}
	for _, peer := range peers {
		profiles[peer] = uuid.New()
	}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxProf := withAccountProfileCtx(ctx, acc, prof)

	chatIDs := make([]string, 0, 16)
	for _, peer := range peers {
		dm, err := client.CreateDM(ctxProf, &chatv1.CreateDMRequest{OtherProfileId: peer.String()})
		require.NoError(t, err)
		chatIDs = append(chatIDs, dm.GetChat().GetId())
	}

	empty, err := client.ListQuickAccess(ctxProf, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Empty(t, empty.GetItems())

	for i := 0; i < 15; i++ {
		_, err = client.AddQuickAccess(ctxProf, &chatv1.AddQuickAccessRequest{ChatId: chatIDs[i]})
		require.NoError(t, err)
	}

	_, err = client.AddQuickAccess(ctxProf, &chatv1.AddQuickAccessRequest{ChatId: chatIDs[15]})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	list, err := client.ListQuickAccess(ctxProf, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetItems(), 15)
	require.NotNil(t, list.GetItems()[0].GetChat())

	_, err = client.AddQuickAccess(ctxProf, &chatv1.AddQuickAccessRequest{ChatId: chatIDs[0]})
	require.NoError(t, err)

	_, err = client.ReorderQuickAccess(ctxProf, &chatv1.ReorderQuickAccessRequest{
		ChatIds: append([]string{chatIDs[2], chatIDs[0], chatIDs[1]}, chatIDs[3:15]...),
	})
	require.NoError(t, err)

	list, err = client.ListQuickAccess(ctxProf, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Equal(t, chatIDs[2], list.GetItems()[0].GetChatId())

	_, err = client.RemoveQuickAccess(ctxProf, &chatv1.RemoveQuickAccessRequest{ChatId: chatIDs[2]})
	require.NoError(t, err)

	list, err = client.ListQuickAccess(ctxProf, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetItems(), 14)
}

func TestQuickAccess_AddNonMember_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	acc := uuid.New()
	prof := uuid.New()
	client, cleanup := startChatGRPCTestServer(t, pool, mapProfileAccounts{prof: acc}, nil, nil)
	t.Cleanup(cleanup)

	_, err := client.AddQuickAccess(withAccountProfileCtx(ctx, acc, prof), &chatv1.AddQuickAccessRequest{
		ChatId: uuid.New().String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

// TestQuickAccess_HidesDeletedPeerDMAndFailsClosed documents PLAN A1: a fresh
// Quick Access snapshot must omit a DM whose peer account was soft-deleted.
// It must not disclose stored shortcuts while the lifecycle gate is unavailable.
func TestQuickAccess_HidesDeletedPeerDMAndFailsClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accountA, profileA := uuid.New(), uuid.New()
	deletedAccount, deletedProfile := uuid.New(), uuid.New()
	activeAccount, activeProfile := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{
		profileA:       accountA,
		deletedProfile: deletedAccount,
		activeProfile:  activeAccount,
	}

	// Seed before the lifecycle gate is connected: after deletion neither
	// CreateDM nor a new message may recreate this relationship.
	deletedDM := seedDMForDeletedPeerTest(t, ctx, pool, profileA, deletedProfile, store.InboxMain)
	activeDM := seedDMForDeletedPeerTest(t, ctx, pool, profileA, activeProfile, store.InboxMain)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithAccountDeletedChecker(mapDeletedAccounts{deletedAccount: {}}))
	t.Cleanup(cleanup)
	caller := withAccountProfileCtx(ctx, accountA, profileA)
	for _, chatID := range []string{deletedDM.ID.String(), activeDM.ID.String()} {
		_, err := client.AddQuickAccess(caller, &chatv1.AddQuickAccessRequest{ChatId: chatID})
		require.NoError(t, err)
	}

	visible, err := client.ListQuickAccess(caller, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Len(t, visible.GetItems(), 1)
	require.Equal(t, activeDM.ID.String(), visible.GetItems()[0].GetChatId())

	failClosedClient, failClosedCleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithAccountDeletedChecker(unavailableDeletedAccounts{}))
	t.Cleanup(failClosedCleanup)
	response, err := failClosedClient.ListQuickAccess(caller, &chatv1.ListQuickAccessRequest{})
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

// TestQuickAccess_DeletedPeerFilterUnit keeps the A1 lifecycle boundary
// executable when the PostgreSQL integration fixture is unavailable locally.
func TestQuickAccess_DeletedPeerFilterUnit(t *testing.T) {
	deletedAccount, deletedProfile := uuid.New(), uuid.New()
	activeAccount, activeProfile := uuid.New(), uuid.New()
	deletedDM, activeDM := uuid.New(), uuid.New()
	service := &ChatGRPC{
		LifecycleOwners: mapProfileAccounts{
			deletedProfile: deletedAccount,
			activeProfile:  activeAccount,
		},
		DeletedAccounts: mapDeletedAccounts{deletedAccount: {}},
	}

	visible, err := service.filterQuickAccessDeletedPeerDMs(context.Background(),
		[]*store.ChatRow{
			{ID: deletedDM, Type: "dm"},
			{ID: activeDM, Type: "dm"},
		},
		map[uuid.UUID]uuid.UUID{deletedDM: deletedProfile, activeDM: activeProfile},
	)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	require.Equal(t, activeDM, visible[0].ID)

	service.DeletedAccounts = unavailableDeletedAccounts{}
	visible, err = service.filterQuickAccessDeletedPeerDMs(context.Background(),
		[]*store.ChatRow{{ID: activeDM, Type: "dm"}},
		map[uuid.UUID]uuid.UUID{activeDM: activeProfile},
	)
	require.Nil(t, visible)
	require.ErrorIs(t, err, errDeletedAccountGateUnavailable)
}

// TestListQuickAccess_DeletedPeerFilterUnit proves the RPC calls the lifecycle
// filter before it serializes the fresh Quick Access snapshot.
func TestListQuickAccess_DeletedPeerFilterUnit(t *testing.T) {
	profileID := uuid.New()
	deletedAccount, deletedProfile := uuid.New(), uuid.New()
	activeAccount, activeProfile := uuid.New(), uuid.New()
	deletedDM, activeDM := uuid.New(), uuid.New()
	store := &quickAccessLifecycleStore{
		rows: []store.QuickAccessRow{
			{ChatID: deletedDM, SortOrder: 0},
			{ChatID: activeDM, SortOrder: 1},
		},
		chats: map[uuid.UUID]*store.ChatRow{
			deletedDM: {ID: deletedDM, Type: "dm"},
			activeDM:  {ID: activeDM, Type: "dm"},
		},
		peers: map[uuid.UUID]uuid.UUID{
			deletedDM: deletedProfile,
			activeDM:  activeProfile,
		},
	}
	service := &ChatGRPC{
		DM: store,
		LifecycleOwners: mapProfileAccounts{
			deletedProfile: deletedAccount,
			activeProfile:  activeAccount,
		},
		DeletedAccounts: mapDeletedAccounts{deletedAccount: {}},
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderProfileID, profileID.String(),
	))

	response, err := service.ListQuickAccess(ctx, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Len(t, response.GetItems(), 1)
	require.Equal(t, activeDM.String(), response.GetItems()[0].GetChatId())

	service.DeletedAccounts = unavailableDeletedAccounts{}
	response, err = service.ListQuickAccess(ctx, &chatv1.ListQuickAccessRequest{})
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

type quickAccessLifecycleStore struct {
	DMStore
	rows  []store.QuickAccessRow
	chats map[uuid.UUID]*store.ChatRow
	peers map[uuid.UUID]uuid.UUID
}

func (s *quickAccessLifecycleStore) ListQuickAccess(context.Context, uuid.UUID) ([]store.QuickAccessRow, error) {
	return s.rows, nil
}

func (s *quickAccessLifecycleStore) FindChatByID(_ context.Context, chatID uuid.UUID) (*store.ChatRow, error) {
	return s.chats[chatID], nil
}

func (s *quickAccessLifecycleStore) DMPeerProfileIDs(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	return s.peers, nil
}
