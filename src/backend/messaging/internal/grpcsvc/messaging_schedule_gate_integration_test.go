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

func TestMessagingSendMessage_scheduleDoesNotCreateImmediateMessageOrEvent(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")

	accountID := uuid.New()
	profileID := uuid.New()
	peerID := uuid.New()
	events := &spyMessageEvents{}
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{MessageEvents: events})
	defer cleanup()

	// BE-077/078 accept pending schedules; BE-079 automatic dispatch is separate.
	// Both accepted and rejected schedule arms must avoid immediate delivery.
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, tc := range []struct {
		name string
		req  *messagingv1.SendMessageRequest
		code codes.Code
	}{
		{"future", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(now.Add(time.Hour))}}, codes.OK},
		{"when online", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: true}}, codes.OK},
		{"past", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(now.Add(-time.Hour))}}, codes.InvalidArgument},
		{"beyond horizon", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(now.Add(366 * 24 * time.Hour))}}, codes.InvalidArgument},
		{"missing timestamp", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{}}, codes.InvalidArgument},
		{"false online", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: false}}, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chatID := uuid.New()
			seedDMChat(t, ctx, pool, chatID, profileID, peerID)
			tc.req.Chat = chatDMRef(chatID)
			tc.req.Content = "must not become immediate"
			response, err := client.SendMessage(withProfileCtx(ctx, accountID, profileID), tc.req)
			require.Equal(t, tc.code, status.Code(err))

			var count int
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM scheduled_messages WHERE chat_id = $1", chatID).Scan(&count))
			if tc.code == codes.OK {
				require.NoError(t, err)
				require.Nil(t, response.Message)
				scheduled := response.GetScheduledMessage()
				require.NotNil(t, scheduled)
				require.Equal(t, messagingv1.ScheduledMessageStatus_SCHEDULED_MESSAGE_STATUS_PENDING, scheduled.Status)
				require.Equal(t, profileID.String(), scheduled.SenderProfileId)
				require.Equal(t, tc.req.Content, scheduled.GetPayload().GetContent())
				require.Equal(t, 1, count)
				var state string
				require.NoError(t, pool.QueryRow(ctx, "SELECT status FROM scheduled_messages WHERE id = $1 AND chat_id = $2 AND sender_profile_id = $3", scheduled.Id, chatID, profileID).Scan(&state))
				require.Equal(t, "pending", state)
				if tc.req.GetScheduledAt() != nil {
					require.True(t, tc.req.GetScheduledAt().AsTime().Equal(scheduled.GetScheduledAt().AsTime()))
				} else {
					require.True(t, scheduled.GetSendWhenOnline())
				}
			} else {
				require.Nil(t, response)
				require.Zero(t, count)
			}
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE chat_id = $1", chatID).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM message_event_outbox").Scan(&count))
			require.Zero(t, count)
			require.Zero(t, events.eventCount())
		})
	}
}
