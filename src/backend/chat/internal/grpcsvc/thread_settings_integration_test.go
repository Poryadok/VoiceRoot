package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
)

type chatThreadSettings struct {
	ThreadsEnabled      bool
	AllowUserMainFeed   bool
}

func readChatThreadSettings(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID string) chatThreadSettings {
	t.Helper()
	var out chatThreadSettings
	err := pool.QueryRow(ctx, `
SELECT threads_enabled, allow_user_main_feed
FROM chats
WHERE id = $1
`, chatID).Scan(&out.ThreadsEnabled, &out.AllowUserMainFeed)
	require.NoError(t, err)
	return out
}

// TestCreateChat_groupThreadDefaults documents text-chat.md group defaults.
func TestCreateChat_groupThreadDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := profileIDs(profiles)
	owner := ids[0]
	inviteeA := ids[1]
	inviteeB := ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	group := createStandaloneGroup(t, client, profiles, owner, "app stack0 group", inviteeA, inviteeB)
	settings := readChatThreadSettings(t, context.Background(), pool, group.GetId())

	require.False(t, settings.ThreadsEnabled, "group threads_enabled default")
	require.True(t, settings.AllowUserMainFeed, "group allow_user_main_feed default")
}

// TestCreateChat_channelThreadDefaults documents text-chat.md channel defaults.
func TestCreateChat_channelThreadDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New())
	owner := uuidKey(profiles)
	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	name := "Announcements"
	spaceID := uuid.New().String()
	resp, err := client.CreateChat(ctxFor(t, profiles, owner), &chatv1.CreateChatRequest{
		Type:    chatv1.ChatType_CHAT_TYPE_CHANNEL,
		Name:    &name,
		SpaceId: &spaceID,
	})
	require.NoError(t, err)

	settings := readChatThreadSettings(t, context.Background(), pool, resp.GetChat().GetId())
	require.True(t, settings.ThreadsEnabled, "channel threads_enabled default")
	require.False(t, settings.AllowUserMainFeed, "channel allow_user_main_feed default")
}
