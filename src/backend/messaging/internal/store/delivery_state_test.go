package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/messageid"
)

func TestDeriveLastMessageDeliveryState(t *testing.T) {
	msg := uuid.MustParse("018f4cde-f345-7abf-bdef-0123456789ab")
	earlier := uuid.MustParse("018f4cde-f345-7abf-bdef-0123456789aa")
	later := uuid.MustParse("018f4cde-f345-7abf-bdef-0123456789ac")

	require.Equal(t, "none", deriveLastMessageDeliveryState(false, msg, nil, nil))
	require.Equal(t, "sent", deriveLastMessageDeliveryState(true, msg, nil, nil))
	require.Equal(t, "delivered", deriveLastMessageDeliveryState(true, msg, nil, &later))
	require.Equal(t, "read", deriveLastMessageDeliveryState(true, msg, &later, nil))
	require.Equal(t, "sent", deriveLastMessageDeliveryState(true, msg, &earlier, &earlier))
}

func TestMessagesStore_UpsertDeliveredCursor_andMetadata(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}

	chatID := uuid.New()
	viewer := uuid.New()
	peer := uuid.New()
	msgPeer, err := messageid.NewMessageID()
	require.NoError(t, err)
	msgOut, err := messageid.NewMessageID()
	require.NoError(t, err)

	_, err = s.InsertMessage(ctx, MessageRow{
		ID: msgPeer, ChatID: chatID, ChatType: "dm", SenderProfileID: peer,
		Content: "peer first", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
	})
	require.NoError(t, err)
	_, err = s.InsertMessage(ctx, MessageRow{
		ID: msgOut, ChatID: chatID, ChatType: "dm", SenderProfileID: viewer,
		Content: "outgoing last", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
	})
	require.NoError(t, err)

	meta, err := s.GetChatListMetadata(ctx, viewer, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.True(t, meta[chatID].LastMessageIsOutgoing)
	require.Equal(t, "sent", meta[chatID].LastMessageDeliveryState)

	require.NoError(t, s.UpsertDeliveredCursor(ctx, chatID, peer, msgOut))
	meta, err = s.GetChatListMetadata(ctx, viewer, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.Equal(t, "delivered", meta[chatID].LastMessageDeliveryState)

	require.NoError(t, s.UpsertReadReceipt(ctx, chatID, peer, msgOut))
	meta, err = s.GetChatListMetadata(ctx, viewer, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.Equal(t, "read", meta[chatID].LastMessageDeliveryState)
}
