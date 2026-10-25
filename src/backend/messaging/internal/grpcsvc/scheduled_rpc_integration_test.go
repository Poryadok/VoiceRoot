package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sync"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestScheduledRPCOwnerLifecycleAndSendNow(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000014_managed_chats.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	chat, a, b := uuid.New(), uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, chat, a, b)
	client, _ := startMessagingServer(t, pool)
	actor := withProfileCtx(ctx, uuid.New(), a)
	peer := withProfileCtx(ctx, uuid.New(), b)
	key := uuid.NewString()
	original := &messagingv1.SendMessageRequest{Chat: chatDMRef(chat), Content: "original", ClientMessageId: &key, DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond))}}
	created, err := client.SendMessage(actor, original)
	require.NoError(t, err)
	require.Nil(t, created.Message)
	require.NotNil(t, created.ScheduledMessage)
	id := created.ScheduledMessage.Id
	var reserved *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT dispatch_message_id FROM scheduled_messages WHERE id=$1`, id).Scan(&reserved))
	require.Nil(t, reserved, "message UUIDv7 is reserved at dispatch, not at schedule creation")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox`).Scan(&count))
	require.Zero(t, count)
	list, err := client.ListScheduledMessages(peer, &messagingv1.ListScheduledMessagesRequest{Chat: chatDMRef(chat)})
	require.NoError(t, err)
	require.Empty(t, list.ScheduledMessages)
	_, err = client.UpdateScheduledMessage(peer, &messagingv1.UpdateScheduledMessageRequest{ScheduledMessageId: id, Payload: &messagingv1.ScheduledMessagePayload{Content: "stolen"}})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = client.CancelScheduledMessage(peer, &messagingv1.CancelScheduledMessageRequest{ScheduledMessageId: id})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = client.SendScheduledMessageNow(peer, &messagingv1.SendScheduledMessageNowRequest{ScheduledMessageId: id})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = client.UpdateScheduledMessage(actor, &messagingv1.UpdateScheduledMessageRequest{ScheduledMessageId: id})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	edited, err := client.UpdateScheduledMessage(actor, &messagingv1.UpdateScheduledMessageRequest{ScheduledMessageId: id, Payload: &messagingv1.ScheduledMessagePayload{Content: "edited", SendSilent: true}})
	require.NoError(t, err)
	require.True(t, proto.Equal(original.GetScheduledAt(), edited.ScheduledMessage.GetScheduledAt()))
	replay, err := client.SendMessage(actor, original)
	require.NoError(t, err)
	require.Equal(t, id, replay.ScheduledMessage.Id)
	require.Equal(t, "edited", replay.ScheduledMessage.Payload.Content)
	changed := proto.Clone(original).(*messagingv1.SendMessageRequest)
	changed.DeliverySchedule = nil
	_, err = client.SendMessage(actor, changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	now, err := client.SendScheduledMessageNow(actor, &messagingv1.SendScheduledMessageNowRequest{ScheduledMessageId: id})
	require.NoError(t, err)
	require.Equal(t, "edited", now.Message.Content)
	var silent bool
	var savedKey *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT send_silent,client_message_id FROM messages WHERE id=$1`, now.Message.Id).Scan(&silent, &savedKey))
	require.True(t, silent)
	require.Nil(t, savedKey)
	require.Equal(t, uuid.Version(7), uuid.MustParse(now.Message.Id).Version())
	again, err := client.SendScheduledMessageNow(actor, &messagingv1.SendScheduledMessageNowRequest{ScheduledMessageId: id})
	require.NoError(t, err)
	require.Equal(t, now.Message.Id, again.Message.Id)
	replay, err = client.SendMessage(actor, original)
	require.NoError(t, err)
	require.Nil(t, replay.Message)
	require.Equal(t, now.Message.Id, replay.ScheduledMessage.GetSentMessageId())
	_, err = client.CancelScheduledMessage(actor, &messagingv1.CancelScheduledMessageRequest{ScheduledMessageId: id})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox`).Scan(&count))
	require.Equal(t, 1, count)

	// Concurrent repeat creates exactly one message/outbox and returns one ID.
	for run := 0; run < 3; run++ {
		schedule, err := client.SendMessage(actor, &messagingv1.SendMessageRequest{Chat: chatDMRef(chat), Content: "race", DeliverySchedule: &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: true}})
		require.NoError(t, err)
		var wg sync.WaitGroup
		results := make(chan *messagingv1.SendScheduledMessageNowResponse, 4)
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := client.SendScheduledMessageNow(actor, &messagingv1.SendScheduledMessageNowRequest{ScheduledMessageId: schedule.ScheduledMessage.Id})
				results <- result
				errs <- err
			}()
		}
		wg.Wait()
		close(results)
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		var message string
		for result := range results {
			if message == "" {
				message = result.Message.Id
			}
			require.Equal(t, message, result.Message.Id)
		}
	}
	// Stable cursors expose only this caller's pending rows.
	for i := 0; i < 3; i++ {
		_, err = client.SendMessage(actor, &messagingv1.SendMessageRequest{Chat: chatDMRef(chat), Content: "page", DeliverySchedule: &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: true}})
		require.NoError(t, err)
	}
	page, err := client.ListScheduledMessages(actor, &messagingv1.ListScheduledMessagesRequest{Chat: chatDMRef(chat), Page: &commonv1.CursorPageRequest{PageSize: 2}})
	require.NoError(t, err)
	require.Len(t, page.ScheduledMessages, 2)
	require.True(t, page.Page.HasMore)
	tail, err := client.ListScheduledMessages(actor, &messagingv1.ListScheduledMessagesRequest{Chat: chatDMRef(chat), Page: &commonv1.CursorPageRequest{PageSize: 2, Cursor: page.Page.NextCursor}})
	require.NoError(t, err)
	require.Len(t, tail.ScheduledMessages, 1)
	require.False(t, tail.Page.HasMore)
	_, err = client.ListScheduledMessages(peer, &messagingv1.ListScheduledMessagesRequest{Chat: chatDMRef(chat), Page: &commonv1.CursorPageRequest{Cursor: page.Page.NextCursor}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
