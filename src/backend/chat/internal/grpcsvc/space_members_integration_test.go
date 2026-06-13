package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"

	"voice/backend/chat/internal/store"
	"voice/backend/pkg/integrationtest"
)

func applySpaceMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}

func seedSpaceChannelChat(t *testing.T, ctx context.Context, chatPool *pgxpool.Pool, spacePool *pgxpool.Pool, chatID, spaceID, owner, member uuid.UUID, name string) {
	t.Helper()
	_, err := chatPool.Exec(ctx, `
INSERT INTO chats (id, type, space_id, name, creator_profile_id, slow_mode_seconds)
VALUES ($1, 'channel', $2, $3, $4, 0)
`, chatID, spaceID, name, owner)
	require.NoError(t, err)
	_, err = spacePool.Exec(ctx, `
INSERT INTO spaces (id, name, visibility, owner_profile_id, member_count)
VALUES ($1, 'test space', 'public', $2, 2)
`, spaceID, owner)
	require.NoError(t, err)
	_, err = spacePool.Exec(ctx, `
INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2), ($1, $3)
`, spaceID, owner, member)
	require.NoError(t, err)
}

// TestListMembers_SpaceChannel_InheritsSpaceMembers documents DATA_MODEL space member inheritance.
func TestListMembers_SpaceChannel_InheritsSpaceMembers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	chatPool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, chatPool)
	spacePool := integrationtest.StartPostgres(t, ctx, "spacedb_members", "")
	applySpaceMigration(t, ctx, spacePool)

	chatID := uuid.New()
	spaceID := uuid.New()
	owner := uuid.New()
	member := uuid.New()
	accOwner := uuid.New()
	seedSpaceChannelChat(t, ctx, chatPool, spacePool, chatID, spaceID, owner, member, "general")

	spaceMembers := &store.SpaceMembersStore{Pool: spacePool}
	client, cleanup := startChatGRPCTestServer(t, chatPool, nil, nil, nil, WithSpaceMembers(spaceMembers))
	t.Cleanup(cleanup)

	r, err := client.ListMembers(withAccountProfileCtx(ctx, accOwner, owner), &chatv1.ListMembersRequest{
		ChatId: chatID.String(),
		Page:   &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	members := r.GetMemberList().GetMembers()
	require.Len(t, members, 2)
	ids := map[string]struct{}{members[0].GetProfileId(): {}, members[1].GetProfileId(): {}}
	require.Contains(t, ids, owner.String())
	require.Contains(t, ids, member.String())

	var chatMemberCount int
	err = chatPool.QueryRow(ctx, `SELECT COUNT(*) FROM chat_members WHERE chat_id = $1`, chatID).Scan(&chatMemberCount)
	require.NoError(t, err)
	require.Zero(t, chatMemberCount)
}

// TestGetChat_SpaceChannel_SpaceMemberAllowed documents space members can open channel without chat_members row.
func TestGetChat_SpaceChannel_SpaceMemberAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	chatPool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, chatPool)
	spacePool := integrationtest.StartPostgres(t, ctx, "spacedb_getchat", "")
	applySpaceMigration(t, ctx, spacePool)

	chatID := uuid.New()
	spaceID := uuid.New()
	owner := uuid.New()
	member := uuid.New()
	accMember := uuid.New()
	seedSpaceChannelChat(t, ctx, chatPool, spacePool, chatID, spaceID, owner, member, "general")

	spaceMembers := &store.SpaceMembersStore{Pool: spacePool}
	client, cleanup := startChatGRPCTestServer(t, chatPool, nil, nil, nil, WithSpaceMembers(spaceMembers))
	t.Cleanup(cleanup)

	r, err := client.GetChat(withAccountProfileCtx(ctx, accMember, member), &chatv1.GetChatRequest{ChatId: chatID.String()})
	require.NoError(t, err)
	require.Equal(t, chatID.String(), r.GetChat().GetId())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, r.GetChat().GetType())
}
