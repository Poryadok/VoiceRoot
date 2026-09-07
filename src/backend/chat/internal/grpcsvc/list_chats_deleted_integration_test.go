package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "voice.app/voice/common/v1"
	"voice/backend/chat/internal/store"

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

type unavailableDeletedAccounts struct{}

func (unavailableDeletedAccounts) DeletedAmong(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return nil, errors.New("auth deleted-account lookup unavailable")
}

type unavailableProfileLookup struct{}

func (unavailableProfileLookup) AccountIDByProfileID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, errors.New("profile lookup unavailable")
}

func (unavailableProfileLookup) IsGuestProfile(context.Context, uuid.UUID) (bool, error) {
	return false, status.Error(codes.Unavailable, "profile lookup unavailable")
}

type peerLookupDMStore struct {
	DMStore
	peers map[uuid.UUID]uuid.UUID
	err   error
}

func (s peerLookupDMStore) DMPeerProfileIDs(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.peers, nil
}

func seedDMForDeletedPeerTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, caller, peer uuid.UUID, peerInbox string) *store.ChatRow {
	t.Helper()
	row, _, err := (&store.DMStore{Pool: pool}).EnsureDM(ctx, caller, peer, peerInbox)
	require.NoError(t, err)
	return row
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
	// Seed the pre-delete inbox before the deleted-account gate is connected.
	seedDMForDeletedPeerTest(t, ctx, pool, profA, profB, store.InboxMain)
	seedDMForDeletedPeerTest(t, ctx, pool, profA, profC, store.InboxMain)

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithAccountDeletedChecker(mapDeletedAccounts{accB: {}}))
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	list, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	items := list.GetChatList().GetItems()
	require.Len(t, items, 1, "deleted-peer DM must be hidden; active DM remains")
	require.Equal(t, profC.String(), items[0].GetDmPeerProfileId())
}

// TestListChats_HidesDeletedPeerDMsFromEveryInbox documents PLAN A1: a fresh
// snapshot must not surface a DM with a deleted peer in main, requests, archive,
// or a custom folder. Existing group/channel rows and active DMs remain visible.
func TestListChats_HidesDeletedPeerDMsFromEveryInbox(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA, profA := uuid.New(), uuid.New()
	accMain, profMain := uuid.New(), uuid.New()
	accRequest, profRequest := uuid.New(), uuid.New()
	accArchive, profArchive := uuid.New(), uuid.New()
	accFolder, profFolder := uuid.New(), uuid.New()
	accActive, profActive := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{
		profA: accA, profMain: accMain, profRequest: accRequest, profArchive: accArchive,
		profFolder: accFolder, profActive: accActive,
	}

	// Seed the pre-delete state directly. The post-delete Chat instance below must
	// never be able to recreate these DMs through CreateDM/GetDM.
	main := seedDMForDeletedPeerTest(t, ctx, pool, profA, profMain, store.InboxMain)
	seedDMForDeletedPeerTest(t, ctx, pool, profRequest, profA, store.InboxRequests)
	archive := seedDMForDeletedPeerTest(t, ctx, pool, profA, profArchive, store.InboxMain)
	folderDM := seedDMForDeletedPeerTest(t, ctx, pool, profA, profFolder, store.InboxMain)
	active := seedDMForDeletedPeerTest(t, ctx, pool, profA, profActive, store.InboxMain)

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithAccountDeletedChecker(mapDeletedAccounts{
			accMain: {}, accRequest: {}, accArchive: {}, accFolder: {},
		}),
	)
	t.Cleanup(cleanup)
	ctxA := withAccountProfileCtx(ctx, accA, profA)

	_, err := client.ArchiveChat(ctxA, &chatv1.ArchiveChatRequest{ChatId: archive.ID.String(), Archived: true})
	require.NoError(t, err)
	folder, err := client.CreateFolder(ctxA, &chatv1.CreateFolderRequest{Name: "Deleted peer"})
	require.NoError(t, err)
	folderID := folder.GetFolder().GetId()
	_, err = client.AddChatToFolder(ctxA, &chatv1.AddChatToFolderRequest{
		FolderId: folderID,
		ChatId:   folderDM.ID.String(),
	})
	require.NoError(t, err)

	mainList, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{Page: &commonv1.CursorPageRequest{PageSize: 100}})
	require.NoError(t, err)
	require.Len(t, mainList.GetChatList().GetItems(), 1)
	require.Equal(t, active.ID.String(), mainList.GetChatList().GetItems()[0].GetChat().GetId())

	requests := "requests"
	requestList, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{Inbox: &requests})
	require.NoError(t, err)
	require.Empty(t, requestList.GetChatList().GetItems(), "deleted-peer request must not appear in a fresh snapshot")

	archiveInbox := "archive"
	archiveList, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{Inbox: &archiveInbox})
	require.NoError(t, err)
	require.Empty(t, archiveList.GetChatList().GetItems(), "deleted-peer archive entry must not appear in a fresh snapshot")

	folderList, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{FolderId: &folderID})
	require.NoError(t, err)
	require.Empty(t, folderList.GetChatList().GetItems(), "deleted-peer folder entry must not appear in a fresh snapshot")

	// Keep the variables explicit: the test documents which old rows are excluded.
	require.NotEqual(t, main.ID, active.ID)
}

// TestListChats_DeletedPeerSnapshotUsesLifecycleOwnerLookup proves the snapshot
// path does not fall back to public GetProfile, which intentionally hides a
// soft-deleted peer. Every ListChats scope must omit that peer only after the
// internal lifecycle owner lookup and Auth gate complete successfully.
func TestListChats_DeletedPeerSnapshotUsesLifecycleOwnerLookup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accountA, profileA := uuid.New(), uuid.New()
	accountDeleted, profileDeleted := uuid.New(), uuid.New()
	accountRequest, profileRequest := uuid.New(), uuid.New()
	accountActive, profileActive := uuid.New(), uuid.New()
	main := seedDMForDeletedPeerTest(t, ctx, pool, profileA, profileDeleted, store.InboxMain)
	seedDMForDeletedPeerTest(t, ctx, pool, profileRequest, profileA, store.InboxRequests)
	active := seedDMForDeletedPeerTest(t, ctx, pool, profileA, profileActive, store.InboxMain)
	archive := seedDMForDeletedPeerTest(t, ctx, pool, profileA, uuid.New(), store.InboxMain)
	archivePeer := mustDMPeer(t, ctx, pool, archive.ID, profileA)
	folderDM := seedDMForDeletedPeerTest(t, ctx, pool, profileA, uuid.New(), store.InboxMain)
	folderPeer := mustDMPeer(t, ctx, pool, folderDM.ID, profileA)

	owners := mapLifecycleOwners{
		profileDeleted: accountDeleted,
		profileRequest: accountRequest,
		profileActive:  accountActive,
		archivePeer:    uuid.New(),
		folderPeer:     uuid.New(),
	}
	deleted := mapDeletedAccounts{
		accountDeleted:      {},
		accountRequest:      {},
		owners[archivePeer]: {},
		owners[folderPeer]:  {},
	}
	client, cleanup := startChatGRPCTestServer(t, pool, unavailableProfileLookup{}, nil, nil,
		WithLifecycleOwnerLookup(owners),
		WithAccountDeletedChecker(deleted),
	)
	t.Cleanup(cleanup)
	caller := withAccountProfileCtx(ctx, accountA, profileA)

	_, err := client.ArchiveChat(caller, &chatv1.ArchiveChatRequest{ChatId: archive.ID.String(), Archived: true})
	require.NoError(t, err)
	folder, err := client.CreateFolder(caller, &chatv1.CreateFolderRequest{Name: "deleted lifecycle peer"})
	require.NoError(t, err)
	folderID := folder.GetFolder().GetId()
	_, err = client.AddChatToFolder(caller, &chatv1.AddChatToFolderRequest{FolderId: folderID, ChatId: folderDM.ID.String()})
	require.NoError(t, err)

	mainList, err := client.ListChats(caller, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, mainList.GetChatList().GetItems(), 1)
	require.Equal(t, active.ID.String(), mainList.GetChatList().GetItems()[0].GetChat().GetId())

	requestsInbox := "requests"
	requestsList, err := client.ListChats(caller, &chatv1.ListChatsRequest{Inbox: &requestsInbox})
	require.NoError(t, err)
	require.Empty(t, requestsList.GetChatList().GetItems())

	archiveInbox := "archive"
	archiveList, err := client.ListChats(caller, &chatv1.ListChatsRequest{Inbox: &archiveInbox})
	require.NoError(t, err)
	require.Empty(t, archiveList.GetChatList().GetItems())

	folderList, err := client.ListChats(caller, &chatv1.ListChatsRequest{FolderId: &folderID})
	require.NoError(t, err)
	require.Empty(t, folderList.GetChatList().GetItems())

	require.NotEqual(t, main.ID, active.ID)
}

func TestListChats_DeletedPeerLifecycleOwnerDependencyFailureIsUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accountCaller, profileCaller := uuid.New(), uuid.New()
	accountPeer, profilePeer := uuid.New(), uuid.New()
	seedDMForDeletedPeerTest(t, ctx, pool, profileCaller, profilePeer, store.InboxMain)
	publicProfiles := mapProfileAccounts{profileCaller: accountCaller, profilePeer: accountPeer}
	client, cleanup := startChatGRPCTestServer(t, pool, publicProfiles, nil, nil,
		WithLifecycleOwnerLookup(unavailableProfileLookup{}),
		WithAccountDeletedChecker(mapDeletedAccounts{}),
	)
	t.Cleanup(cleanup)

	response, err := client.ListChats(withAccountProfileCtx(ctx, accountCaller, profileCaller), &chatv1.ListChatsRequest{})
	require.Error(t, err)
	require.Nil(t, response, "lifecycle owner failure must not become an unfiltered or empty snapshot")
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func mustDMPeer(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, viewerID uuid.UUID) uuid.UUID {
	t.Helper()
	peers, err := (&store.DMStore{Pool: pool}).DMPeerProfileIDs(ctx, viewerID, []uuid.UUID{chatID})
	require.NoError(t, err)
	peerID, ok := peers[chatID]
	require.True(t, ok)
	return peerID
}

func TestListChats_DeletedPeerFilteringPreservesActiveDMGroupAndChannel(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA, profA := uuid.New(), uuid.New()
	accDeleted, profDeleted := uuid.New(), uuid.New()
	accActive, profActive := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{profA: accA, profDeleted: accDeleted, profActive: accActive}
	deletedDM := seedDMForDeletedPeerTest(t, ctx, pool, profA, profDeleted, store.InboxMain)
	activeDM := seedDMForDeletedPeerTest(t, ctx, pool, profA, profActive, store.InboxMain)

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithAccountDeletedChecker(mapDeletedAccounts{accDeleted: {}}),
	)
	t.Cleanup(cleanup)
	ctxA := withAccountProfileCtx(ctx, accA, profA)
	groupName, channelName := "Still here group", "Still here channel"
	group, err := client.CreateChat(ctxA, &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, Name: &groupName})
	require.NoError(t, err)
	channel, err := client.CreateChat(ctxA, &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_CHANNEL, Name: &channelName})
	require.NoError(t, err)

	list, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{Page: &commonv1.CursorPageRequest{PageSize: 100}})
	require.NoError(t, err)
	gotTypes := map[string]chatv1.ChatType{}
	for _, item := range list.GetChatList().GetItems() {
		gotTypes[item.GetChat().GetId()] = item.GetChat().GetType()
	}
	require.NotContains(t, gotTypes, deletedDM.ID.String())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, gotTypes[activeDM.ID.String()])
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, gotTypes[group.GetChat().GetId()])
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, gotTypes[channel.GetChat().GetId()])
}

// TestListChats_DeletedPeerFilteringPreservesCursorContinuation documents the
// raw-page cursor contract: a fully filtered page stays empty but its cursor
// must lead to the next active row instead of terminating the snapshot early.
func TestListChats_DeletedPeerFilteringPreservesCursorContinuation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA, profA := uuid.New(), uuid.New()
	accDeletedOne, profDeletedOne := uuid.New(), uuid.New()
	accDeletedTwo, profDeletedTwo := uuid.New(), uuid.New()
	accActive, profActive := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{
		profA: accA, profDeletedOne: accDeletedOne, profDeletedTwo: accDeletedTwo, profActive: accActive,
	}
	deletedOne := seedDMForDeletedPeerTest(t, ctx, pool, profA, profDeletedOne, store.InboxMain)
	deletedTwo := seedDMForDeletedPeerTest(t, ctx, pool, profA, profDeletedTwo, store.InboxMain)
	active := seedDMForDeletedPeerTest(t, ctx, pool, profA, profActive, store.InboxMain)
	chatStore := &store.DMStore{Pool: pool}
	now := time.Now().UTC()
	require.NoError(t, chatStore.TouchLastMessageAt(ctx, deletedOne.ID, now.Add(3*time.Second)))
	require.NoError(t, chatStore.TouchLastMessageAt(ctx, deletedTwo.ID, now.Add(2*time.Second)))
	require.NoError(t, chatStore.TouchLastMessageAt(ctx, active.ID, now.Add(time.Second)))

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithAccountDeletedChecker(mapDeletedAccounts{accDeletedOne: {}, accDeletedTwo: {}}),
	)
	t.Cleanup(cleanup)
	ctxA := withAccountProfileCtx(ctx, accA, profA)
	first, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	require.Empty(t, first.GetChatList().GetItems(), "the first raw page contains only deleted-peer DMs")
	require.NotEmpty(t, first.GetChatList().GetNextCursor(), "the client must be able to continue after a fully filtered raw page")

	second, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 2, Cursor: first.GetChatList().GetNextCursor()},
	})
	require.NoError(t, err)
	require.Len(t, second.GetChatList().GetItems(), 1)
	require.Equal(t, active.ID.String(), second.GetChatList().GetItems()[0].GetChat().GetId())
	require.Empty(t, second.GetChatList().GetNextCursor())
}

// TestListChats_DeletedPeerGateFailuresAreUnavailable prevents freshness errors
// from becoming an unfiltered snapshot or a deceptive empty one.
func TestListChats_DeletedPeerGateFailuresAreUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA, profA := uuid.New(), uuid.New()
	accB, profB := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}
	seedDMForDeletedPeerTest(t, ctx, pool, profA, profB, store.InboxMain)
	ctxA := withAccountProfileCtx(ctx, accA, profA)

	tests := []struct {
		name     string
		profiles UserProfileLookup
		deleted  AccountDeletedChecker
	}{
		{name: "missing_auth_checker", profiles: profiles},
		{name: "missing_profile_lookup", deleted: mapDeletedAccounts{}},
		{name: "auth_checker_failure", profiles: profiles, deleted: unavailableDeletedAccounts{}},
		{name: "profile_lookup_failure", profiles: unavailableProfileLookup{}, deleted: mapDeletedAccounts{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := []chatServerOption{WithAccountDeletedChecker(tt.deleted)}
			client, cleanup := startChatGRPCTestServer(t, pool, tt.profiles, nil, nil, options...)
			t.Cleanup(cleanup)

			response, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{})
			require.Error(t, err)
			require.Nil(t, response, "a dependency failure must not masquerade as an empty snapshot")
			require.Equal(t, codes.Unavailable, status.Code(err))
		})
	}
}

func TestListChats_DeletedPeerMappingFailuresAreUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accCaller, profCaller := uuid.New(), uuid.New()
	accPeer, profPeer := uuid.New(), uuid.New()
	row := seedDMForDeletedPeerTest(t, ctx, pool, profCaller, profPeer, store.InboxMain)
	profiles := mapProfileAccounts{profCaller: accCaller, profPeer: accPeer}
	base := &store.DMStore{Pool: pool}

	tests := []struct {
		name  string
		store DMStore
	}{
		{
			name:  "peer_mapping_error",
			store: peerLookupDMStore{DMStore: base, err: errors.New("peer lookup unavailable")},
		},
		{
			name:  "peer_mapping_omits_dm",
			store: peerLookupDMStore{DMStore: base, peers: map[uuid.UUID]uuid.UUID{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithDMStore(tt.store))
			t.Cleanup(cleanup)

			response, err := client.ListChats(withAccountProfileCtx(ctx, accCaller, profCaller), &chatv1.ListChatsRequest{})
			require.Error(t, err)
			require.Nil(t, response, "an unresolved DM peer must not leak in a snapshot")
			require.Equal(t, codes.Unavailable, status.Code(err))
		})
	}
	require.NotEqual(t, uuid.Nil, row.ID)
}
