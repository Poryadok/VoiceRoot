package grpcsvc

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

// This consumes observed rows from the actual Chat composite PG test. The SQL
// fixture and permitted Role/member controls isolate production SendMessage's
// main-feed policy; this is not live Chat RPC or qualified Space ACL evidence.
func TestSpaceCompositeObservedMainFeedProjectionSendMessage(t *testing.T) {
	path := os.Getenv("BE131_COMPOSITE_MAINFEED_PROJECTION")
	if testing.Short() || path == "" {
		t.Skip("requires actual Chat composite PG projection artifact")
	}
	var input struct {
		SourceSHA string
		Chats     []struct {
			ID                                uuid.UUID
			Type                              string
			SpaceID                           *uuid.UUID
			CreatorProfileID                  uuid.UUID
			ThreadsEnabled, AllowUserMainFeed bool
		}
	}
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &input))
	require.NotEmpty(t, input.SourceSHA)
	require.Equal(t, os.Getenv("BE131_COMPOSITE_SOURCE_SHA"), input.SourceSHA)
	require.Len(t, input.Chats, 2)
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)
	client, stop := startMessagingServerWired(t, pool, messagingWire{RolePermissions: testRolePerms{allowed: true}})
	defer stop()
	for _, row := range input.Chats {
		t.Run(row.Type, func(t *testing.T) {
			require.NotNil(t, row.SpaceID)
			require.NotEqual(t, uuid.Nil, row.ID)
			require.NotEqual(t, uuid.Nil, row.CreatorProfileID)
			_, err := pool.Exec(ctx, `INSERT INTO chats(id,type,space_id,creator_profile_id,threads_enabled,allow_user_main_feed)
			 VALUES($1,$2,$3,$4,$5,$6)`, row.ID, row.Type, row.SpaceID, row.CreatorProfileID, row.ThreadsEnabled, row.AllowUserMainFeed)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, "INSERT INTO chat_members(chat_id,profile_id,role) VALUES($1,$2,'owner')", row.ID, row.CreatorProfileID)
			require.NoError(t, err)
			policy, err := (&store.SQLChatThreadPolicy{Pool: pool}).Load(ctx, row.ID)
			require.NoError(t, err)
			require.Equal(t, row.AllowUserMainFeed, policy.AllowUserMainFeed)
			if row.Type == "channel" {
				require.False(t, policy.AllowUserMainFeed)
				return
			}
			require.Equal(t, "group", row.Type)
			req := &messagingv1.SendMessageRequest{Chat: chatGroupRef(row.ID), Content: "composite group main feed", AttachmentsJson: "[]", MentionsJson: "[]"}
			requestCtx := withProfileCtx(ctx, uuid.New(), row.CreatorProfileID)
			response, err := client.SendMessage(requestCtx, req)
			require.NoError(t, err, "production SendMessage must accept the actual composite GROUP creation projection")
			require.NotEmpty(t, response.GetMessage().GetId())
			var count int
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE chat_id=$1", row.ID).Scan(&count))
			require.Equal(t, 1, count)
			_, err = pool.Exec(ctx, "UPDATE chats SET allow_user_main_feed=false WHERE id=$1", row.ID)
			require.NoError(t, err)
			_, err = client.SendMessage(requestCtx, req)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.ErrorContains(t, err, "main-feed", "negative control reaches the actual main-feed gate")
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE chat_id=$1", row.ID).Scan(&count))
			require.Equal(t, 1, count, "denial precedes another message insert")
		})
	}
}
