package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

func seedDMChatRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, profA, profB uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds)
VALUES ($1, 'dm', $2, 0)
`, chatID, profA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES
  ($1, $2, 'member'),
  ($1, $3, 'member')
`, chatID, profA, profB)
	require.NoError(t, err)
}

func TestListMembers_DM_TwoMembers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	accA := uuid.New()
	seedDMChatRows(t, ctx, pool, chatID, profA, profB)

	client, cleanup := startChatGRPCTestServer(t, pool, nil, nil, nil)
	t.Cleanup(cleanup)

	r, err := client.ListMembers(withAccountProfileCtx(ctx, accA, profA), &chatv1.ListMembersRequest{
		ChatId: chatID.String(),
		Page:   &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	members := r.GetMemberList().GetMembers()
	require.Len(t, members, 2)
	ids := map[string]struct{}{members[0].GetProfileId(): {}, members[1].GetProfileId(): {}}
	require.Contains(t, ids, profA.String())
	require.Contains(t, ids, profB.String())
}

func TestListMembers_NonMember_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profStranger := uuid.New()
	accStranger := uuid.New()
	seedDMChatRows(t, ctx, pool, chatID, profA, profB)

	client, cleanup := startChatGRPCTestServer(t, pool, nil, nil, nil)
	t.Cleanup(cleanup)

	_, err := client.ListMembers(withAccountProfileCtx(ctx, accStranger, profStranger), &chatv1.ListMembersRequest{
		ChatId: chatID.String(),
		Page:   &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestListMembers_UnknownChat_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	acc := uuid.New()
	prof := uuid.New()
	client, cleanup := startChatGRPCTestServer(t, pool, nil, nil, nil)
	t.Cleanup(cleanup)

	_, err := client.ListMembers(withAccountProfileCtx(ctx, acc, prof), &chatv1.ListMembersRequest{
		ChatId: uuid.New().String(),
		Page:   &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetChat_DM_MemberSeesChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	accA := uuid.New()
	seedDMChatRows(t, ctx, pool, chatID, profA, profB)

	client, cleanup := startChatGRPCTestServer(t, pool, nil, nil, nil)
	t.Cleanup(cleanup)

	r, err := client.GetChat(withAccountProfileCtx(ctx, accA, profA), &chatv1.GetChatRequest{ChatId: chatID.String()})
	require.NoError(t, err)
	require.Equal(t, chatID.String(), r.GetChat().GetId())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, r.GetChat().GetType())
}
