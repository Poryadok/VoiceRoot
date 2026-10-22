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
	"voice/backend/chat/internal/store"
)

// seedSpaceChatForPolicyTest creates an existing Chat-owned row for storage and
// downstream policy tests. It is not public composite creation: these tests have
// no Space authority, tree receipt or audit worker and must not fabricate one.
func seedSpaceChatForPolicyTest(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID, spaceID string, kind chatv1.ChatType, name string) *chatv1.Chat {
	t.Helper()
	ctx := context.Background()
	dm := &store.DMStore{Pool: pool}
	var row *store.ChatRow
	var err error
	if kind == chatv1.ChatType_CHAT_TYPE_CHANNEL {
		row, err = dm.CreateSpaceChannelChat(ctx, owner, uuid.MustParse(spaceID), name, nil)
	} else {
		row, err = dm.CreateSpaceGroupChat(ctx, owner, uuid.MustParse(spaceID), name, nil)
	}
	require.NoError(t, err)
	row, err = dm.FindChatByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	return chatRowToProto(row)
}

func TestCreateChat_SpaceRequiresCompositeBeforeWrite(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	owner := uuid.New()
	profiles := profileMap(owner)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)
	name, spaceID := "no fabricated Space authority", uuid.NewString()
	for _, kind := range []chatv1.ChatType{chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL} {
		response, err := client.CreateChat(ctxFor(t, profiles, owner), &chatv1.CreateChatRequest{Type: kind, Name: &name, SpaceId: &spaceID})
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Nil(t, response)
	}
	var rows int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats").Scan(&rows))
	require.Zero(t, rows, "signed callers cannot bypass missing composite authority")
}
