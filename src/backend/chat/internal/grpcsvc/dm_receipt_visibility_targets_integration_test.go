package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

func messagingInternalContext(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "messaging")
}

func TestListDMReceiptVisibilityTargets_MessagingOnlyPaginatesAllDMInboxes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	account, profile := uuid.New(), uuid.New()
	peerMain, peerRequest, peerArchive, peerMalformed := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	profiles := mapProfileAccounts{
		profile: account, peerMain: uuid.New(), peerRequest: uuid.New(), peerArchive: uuid.New(), peerMalformed: uuid.New(),
	}
	dmStore := &store.DMStore{Pool: pool}
	main, _, err := dmStore.EnsureDM(ctx, profile, peerMain, store.InboxMain)
	require.NoError(t, err)
	request, _, err := dmStore.EnsureDM(ctx, peerRequest, profile, store.InboxRequests)
	require.NoError(t, err)
	archive, _, err := dmStore.EnsureDM(ctx, profile, peerArchive, store.InboxMain)
	require.NoError(t, err)
	require.NoError(t, dmStore.SetMemberArchived(ctx, archive.ID, profile, true))
	malformed, _, err := dmStore.EnsureDM(ctx, profile, peerMalformed, store.InboxMain)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket) VALUES ($1, $2, 'member', 'main')`, malformed.ID, uuid.New())
	require.NoError(t, err)
	group, err := dmStore.CreateGroupChat(ctx, profile, "not a dm", nil)
	require.NoError(t, err)
	_, err = dmStore.AddGroupMembers(ctx, group.ID, []uuid.UUID{uuid.New(), uuid.New()})
	require.NoError(t, err)

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)
	caller := messagingInternalContext(ctx)
	page1, err := client.ListDMReceiptVisibilityTargets(caller, &chatv1.ListDMReceiptVisibilityTargetsRequest{
		ProfileId: profile.String(), Page: &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	require.Len(t, page1.GetTargets(), 2)
	require.NotEmpty(t, page1.GetNextCursor())
	page2, err := client.ListDMReceiptVisibilityTargets(caller, &chatv1.ListDMReceiptVisibilityTargetsRequest{
		ProfileId: profile.String(), Page: &commonv1.CursorPageRequest{Cursor: page1.GetNextCursor(), PageSize: 2},
	})
	require.NoError(t, err)
	require.Len(t, page2.GetTargets(), 1)
	require.Empty(t, page2.GetNextCursor())

	got := map[string]string{}
	for _, target := range append(page1.GetTargets(), page2.GetTargets()...) {
		got[target.GetChatId()] = target.GetPeerProfileId()
	}
	require.Equal(t, map[string]string{
		main.ID.String():    peerMain.String(),
		request.ID.String(): peerRequest.String(),
		archive.ID.String(): peerArchive.String(),
	}, got)
	require.NotContains(t, got, group.ID.String())
	require.NotContains(t, got, malformed.ID.String())
}

func TestListDMReceiptVisibilityTargets_RejectsNonMessagingCallers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	client, cleanup := startChatGRPCTestServer(t, pool, nil, nil, nil)
	t.Cleanup(cleanup)

	for _, caller := range []context.Context{
		ctx,
		metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "gateway"),
		metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "messaging", authctx.HeaderInternalCaller, "gateway"),
	} {
		_, err := client.ListDMReceiptVisibilityTargets(caller, &chatv1.ListDMReceiptVisibilityTargetsRequest{ProfileId: uuid.NewString()})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
}

type failingDMReceiptVisibilityStore struct {
	DMStore
}

func (f failingDMReceiptVisibilityStore) ListDMReceiptVisibilityTargets(context.Context, uuid.UUID, string, int) ([]store.DMReceiptVisibilityTarget, string, error) {
	return nil, "", errors.New("chat db unavailable")
}

func TestListDMReceiptVisibilityTargets_DependencyFailureFailsClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	client, cleanup := startChatGRPCTestServer(t, pool, nil, nil, nil,
		WithDMStore(failingDMReceiptVisibilityStore{DMStore: &store.DMStore{Pool: pool}}),
	)
	t.Cleanup(cleanup)
	response, err := client.ListDMReceiptVisibilityTargets(messagingInternalContext(ctx), &chatv1.ListDMReceiptVisibilityTargetsRequest{ProfileId: uuid.NewString()})
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestListDMReceiptVisibilityTargets_InvalidCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	client, cleanup := startChatGRPCTestServer(t, pool, nil, nil, nil)
	t.Cleanup(cleanup)

	_, err := client.ListDMReceiptVisibilityTargets(messagingInternalContext(ctx), &chatv1.ListDMReceiptVisibilityTargetsRequest{
		ProfileId: uuid.NewString(),
		Page:      &commonv1.CursorPageRequest{Cursor: "not-an-opaque-cursor"},
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
