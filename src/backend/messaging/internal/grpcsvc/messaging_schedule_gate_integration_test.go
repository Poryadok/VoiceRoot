package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	messagingv1 "voice.app/voice/messaging/v1"
)

func TestMessagingSendMessage_scheduleIsRejectedWithoutMessageOrEvent(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")

	chatID := uuid.New()
	accountID := uuid.New()
	profileID := uuid.New()
	peerID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profileID, peerID)
	events := &spyMessageEvents{}
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{MessageEvents: events})
	defer cleanup()

	_, err := client.SendMessage(withProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{
		Chat:    chatDMRef(chatID),
		Content: "must not become immediate",
		DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{
			ScheduledAt: timestamppb.New(time.Now().Add(time.Hour)),
		},
	})
	require.Equal(t, codes.Unimplemented, status.Code(err))

	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE chat_id = $1", chatID).Scan(&count))
	require.Zero(t, count)
	require.Zero(t, events.eventCount())
}
