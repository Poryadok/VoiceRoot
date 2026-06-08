package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/messageid"
)

func TestMessagesStore_nilPool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s *MessagesStore
	_, err := s.MessageExists(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
	_, err = s.GetByClientDedupKey(ctx, uuid.New(), uuid.New(), uuid.New())
	require.Error(t, err)
	_, err = s.InsertMessage(ctx, MessageRow{})
	require.Error(t, err)
	_, err = s.GetMessageByID(ctx, uuid.New())
	require.Error(t, err)
	_, err = s.UpdateMessageContent(ctx, uuid.New(), uuid.New(), "x")
	require.Error(t, err)
	err = s.SoftDeleteMessage(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
	err = s.HideMessageForProfile(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
	_, err = s.ListMessages(ctx, uuid.New(), uuid.New(), ListLatest, nil, 10)
	require.Error(t, err)
	err = s.UpsertReadReceipt(ctx, uuid.New(), uuid.New(), uuid.New())
	require.Error(t, err)
	_, _, err = s.GetReadReceipt(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
	_, err = s.GetChatListMetadata(ctx, uuid.New(), nil)
	require.Error(t, err)
}

func TestMessagesStore_CRUDAndList(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}

	chatID := uuid.New()
	sender := uuid.New()
	clientID := uuid.New()
	msgID, err := messageid.NewMessageID()
	require.NoError(t, err)

	row := MessageRow{
		ID:              msgID,
		ChatID:          chatID,
		SenderProfileID: sender,
		Content:         "hello",
		Type:            "regular",
		AttachmentsJSON: "[]",
		MentionsJSON:    "[]",
		ClientMessageID: &clientID,
	}
	saved, err := s.InsertMessage(ctx, row)
	require.NoError(t, err)
	require.Equal(t, msgID, saved.ID)

	got, err := s.GetByClientDedupKey(ctx, chatID, sender, clientID)
	require.NoError(t, err)
	require.Equal(t, msgID, got.ID)

	exists, err := s.MessageExists(ctx, chatID, msgID)
	require.NoError(t, err)
	require.True(t, exists)

	updated, err := s.UpdateMessageContent(ctx, msgID, sender, "revised")
	require.NoError(t, err)
	require.Equal(t, "revised", updated.Content)
	require.NotNil(t, updated.EditedAt)

	require.NoError(t, s.SoftDeleteMessage(ctx, msgID, sender))
	err = s.SoftDeleteMessage(ctx, msgID, sender)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	_, err = s.GetMessageByID(ctx, msgID)
	require.NoError(t, err) // soft-deleted row still fetchable by id
}

func TestMessagesStore_InsertValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}

	chatID := uuid.New()
	sender := uuid.New()
	msgID, err := messageid.NewMessageID()
	require.NoError(t, err)

	_, err = s.InsertMessage(ctx, MessageRow{
		ID: msgID, ChatID: chatID, SenderProfileID: sender,
		AttachmentsJSON: "not-json", MentionsJSON: "[]",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "attachments_json")

	_, err = s.InsertMessage(ctx, MessageRow{
		ID: msgID, ChatID: chatID, SenderProfileID: sender,
		AttachmentsJSON: "[]", MentionsJSON: "{bad",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mentions_json")

	_, err = s.UpdateMessageContent(ctx, uuid.New(), sender, "nope")
	require.ErrorIs(t, err, pgx.ErrNoRows)

	err = s.SoftDeleteMessage(ctx, uuid.New(), sender)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	exists, err := s.MessageExists(ctx, chatID, uuid.New())
	require.NoError(t, err)
	require.False(t, exists)
}

func TestMessagesStore_ListModesAndHides(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}

	chatID := uuid.New()
	sender := uuid.New()
	viewer := uuid.New()
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		msgID, err := messageid.NewMessageID()
		require.NoError(t, err)
		ids = append(ids, msgID)
		_, err = s.InsertMessage(ctx, MessageRow{
			ID: msgID, ChatID: chatID, SenderProfileID: sender,
			Content: "m", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond) // distinct created_at ordering
	}

	latest, err := s.ListMessages(ctx, chatID, viewer, ListLatest, nil, 2)
	require.NoError(t, err)
	require.Len(t, latest, 3) // store fetches limit+1 for paging at gRPC layer

	mid := ids[1]
	before, err := s.ListMessages(ctx, chatID, viewer, ListBeforeID, &mid, 10)
	require.NoError(t, err)
	require.NotEmpty(t, before)

	after, err := s.ListMessages(ctx, chatID, viewer, ListAfterID, &ids[0], 10)
	require.NoError(t, err)
	require.NotEmpty(t, after)

	_, err = s.ListMessages(ctx, chatID, viewer, ListMode(99), nil, 10)
	require.Error(t, err)

	require.NoError(t, s.HideMessageForProfile(ctx, ids[2], viewer))
	require.NoError(t, s.HideMessageForProfile(ctx, ids[2], viewer))

	hidden, err := s.ListMessages(ctx, chatID, viewer, ListLatest, nil, 10)
	require.NoError(t, err)
	for _, m := range hidden {
		require.NotEqual(t, ids[2], m.ID)
	}
}

func TestMessagesStore_ReadReceiptsAndMetadata(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}

	chatID := uuid.New()
	sender := uuid.New()
	viewer := uuid.New()

	lid, upd, err := s.GetReadReceipt(ctx, chatID, viewer)
	require.NoError(t, err)
	require.Nil(t, lid)
	require.Nil(t, upd)

	peerMsg, err := messageid.NewMessageID()
	require.NoError(t, err)
	_, err = s.InsertMessage(ctx, MessageRow{
		ID: peerMsg, ChatID: chatID, SenderProfileID: sender,
		Content: strings.Repeat("а", 200), Type: "regular",
		AttachmentsJSON: "[]", MentionsJSON: "[]",
	})
	require.NoError(t, err)

	meta, err := s.GetChatListMetadata(ctx, viewer, []uuid.UUID{chatID})
	require.NoError(t, err)
	row := meta[chatID]
	require.NotEmpty(t, row.LastMessagePreview)
	require.Len(t, []rune(row.LastMessagePreview), 160)
	require.Equal(t, int64(1), row.UnreadCount)

	require.NoError(t, s.UpsertReadReceipt(ctx, chatID, viewer, peerMsg))
	lid, upd, err = s.GetReadReceipt(ctx, chatID, viewer)
	require.NoError(t, err)
	require.NotNil(t, lid)
	require.NotNil(t, upd)
	require.Equal(t, peerMsg, *lid)
}

func TestMessagesStore_closedPoolErrors(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	pool.Close()
	s := &MessagesStore{Pool: pool}

	_, err := s.MessageExists(ctx, uuid.New(), uuid.New())
	require.Error(t, err)

	_, err = s.InsertMessage(ctx, MessageRow{
		ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: uuid.New(),
		AttachmentsJSON: "[]", MentionsJSON: "[]",
	})
	require.Error(t, err)
}

func TestMessagesStore_SoftDeleteExecError(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	msgID := uuid.New()
	sender := uuid.New()
	pool.Close()
	err := s.SoftDeleteMessage(ctx, msgID, sender)
	require.Error(t, err)
}

func TestTruncatePreview(t *testing.T) {
	t.Parallel()
	short := "hello"
	require.Equal(t, short, truncatePreview(short))
	long := strings.Repeat("а", 200)
	require.Len(t, []rune(truncatePreview(long)), 160)
}
