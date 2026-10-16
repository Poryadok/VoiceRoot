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

func TestMessagesStore_CRUD(t *testing.T) {
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

func TestInsertGameEventMessageIsIdempotentAndExpirySafe(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	store := &MessagesStore{Pool: pool}
	clientID := uuid.New()
	row := MessageRow{ID: uuid.New(), ChatID: uuid.New(), ChatType: "group", SenderProfileID: uuid.New(),
		Content: "A creature was discovered.", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
		ClientMessageID: &clientID, ContentType: "text"}
	expires := time.Now().UTC().Add(time.Minute)
	first, expired, inserted, err := store.InsertGameEventMessage(ctx, row, &expires)
	require.NoError(t, err)
	require.False(t, expired)
	require.True(t, inserted)
	require.NotNil(t, first)
	require.Equal(t, row.ID, first.ID)

	retry := row
	retry.ID = uuid.New()
	oldExpiry := time.Now().UTC().Add(-time.Minute)
	duplicate, expired, inserted, err := store.InsertGameEventMessage(ctx, retry, &oldExpiry)
	require.NoError(t, err)
	require.False(t, expired, "a committed message remains replayable after expiry")
	require.False(t, inserted)
	require.Equal(t, first.ID, duplicate.ID)

	changed := retry
	changed.Content = "A different event payload."
	_, _, _, err = store.InsertGameEventMessage(ctx, changed, nil)
	require.ErrorIs(t, err, ErrGameEventMessageConflict)

	unseen := row
	unseen.ID, unseen.ChatID, unseen.ClientMessageID = uuid.New(), uuid.New(), ptrUUID(uuid.New())
	_, expired, inserted, err = store.InsertGameEventMessage(ctx, unseen, &oldExpiry)
	require.NoError(t, err)
	require.True(t, expired)
	require.False(t, inserted)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE client_message_id=$1`, *unseen.ClientMessageID).Scan(&count))
	require.Zero(t, count)
}

func TestGameCardPersistsAtomicallyAndReplaysImmutablePayload(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	store := &MessagesStore{Pool: pool}
	clientID := uuid.New()
	card := `{"actions":[{"action_id":"00000000-0000-4000-8000-000000000020","action_type":"relic.inspect","arguments_json":"{\"target\":\"relic\"}","label":"Inspect"}],"facts":[{"label":"Location","value":"North gate"}],"media_reference_ids":["00000000-0000-4000-8000-000000000030"],"revision":"1","safe_summary":"A relic was found","schema_version":1,"title":"Relic found"}`
	row := MessageRow{ID: uuid.New(), ChatID: uuid.New(), ChatType: "group", SenderProfileID: uuid.New(),
		Content: "A relic was discovered.", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
		ClientMessageID: &clientID, ContentType: "text", GameCardJSON: &card,
		GameAppID: ptrUUID(uuid.New()), GameEnvironmentID: ptrUUID(uuid.New()), GameInstallationID: ptrUUID(uuid.New()), GameBotID: ptrUUID(uuid.New()),
		GameCharacterBindingID: ptrUUID(uuid.New())}
	expires := time.Now().UTC().Add(time.Minute)
	first, expired, inserted, err := store.InsertGameEventMessage(ctx, row, &expires)
	require.NoError(t, err)
	require.False(t, expired)
	require.True(t, inserted)
	require.True(t, sameStringPointer(&card, first.GameCardJSON))
	require.Len(t, first.GameCardSHA256, 64)
	require.False(t, first.GameCardActionsEnabled)

	read, err := store.GetMessageByID(ctx, row.ID)
	require.NoError(t, err)
	require.True(t, sameStringPointer(&card, read.GameCardJSON))
	require.Equal(t, first.GameCardSHA256, read.GameCardSHA256)
	require.Equal(t, *row.GameAppID, *read.GameAppID)
	require.Equal(t, *row.GameCharacterBindingID, *read.GameCharacterBindingID)
	require.False(t, read.GameCardActionsEnabled)
	_, err = store.UpdateMessageContent(ctx, row.ID, row.SenderProfileID, "replace the card")
	require.ErrorIs(t, err, pgx.ErrNoRows, "game-card fallback and payload remain immutable under the generic edit path")

	retry := row
	retry.ID = uuid.New()
	replayed, expired, inserted, err := store.InsertGameEventMessage(ctx, retry, nil)
	require.NoError(t, err)
	require.False(t, expired)
	require.False(t, inserted)
	require.Equal(t, row.ID, replayed.ID)

	changed := retry
	changed.GameCardJSON = ptrStringForTest(`{"revision":"2"}`)
	_, _, _, err = store.InsertGameEventMessage(ctx, changed, nil)
	require.ErrorIs(t, err, ErrGameEventMessageConflict)
}

func ptrStringForTest(value string) *string { return &value }

func ptrUUID(value uuid.UUID) *uuid.UUID { return &value }

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

func TestTruncatePreview_stripsMarkdown(t *testing.T) {
	t.Parallel()
	require.Equal(t, "bold", truncatePreview("**bold**"))
}

func TestMessagesStore_GetChatListMetadata_stripsMarkdownPreview(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}

	chatID := uuid.New()
	sender := uuid.New()
	viewer := uuid.New()

	msgID, err := messageid.NewMessageID()
	require.NoError(t, err)
	_, err = s.InsertMessage(ctx, MessageRow{
		ID: msgID, ChatID: chatID, SenderProfileID: sender,
		Content: "**hello**", Type: "regular",
		AttachmentsJSON: "[]", MentionsJSON: "[]",
	})
	require.NoError(t, err)

	meta, err := s.GetChatListMetadata(ctx, viewer, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.Equal(t, "hello", meta[chatID].LastMessagePreview)
}

func TestMessagesStore_UnreadMessageSendersDeduplicatesAcrossChatsAndFailsClosedAtLimit(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}

	viewer, repeatedSender, otherSender := uuid.New(), uuid.New(), uuid.New()
	chatA, chatB := uuid.New(), uuid.New()
	for _, chat := range []uuid.UUID{chatA, chatB} {
		msgID, err := messageid.NewMessageID()
		require.NoError(t, err)
		_, err = s.InsertMessage(ctx, MessageRow{
			ID: msgID, ChatID: chat, SenderProfileID: repeatedSender,
			Content: "unread", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
		})
		require.NoError(t, err)
	}
	got, err := s.UnreadMessageSenders(ctx, viewer, []uuid.UUID{chatA, chatB}, 2)
	require.NoError(t, err, "the same profile in multiple chats consumes one request-wide decision")
	require.Equal(t, []uuid.UUID{repeatedSender}, got)

	msgID, err := messageid.NewMessageID()
	require.NoError(t, err)
	_, err = s.InsertMessage(ctx, MessageRow{
		ID: msgID, ChatID: chatB, SenderProfileID: otherSender,
		Content: "unread", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
	})
	require.NoError(t, err)
	_, err = s.UnreadMessageSenders(ctx, viewer, []uuid.UUID{chatA, chatB}, 2)
	require.ErrorIs(t, err, ErrVisibilityCandidateBudgetExceeded)
}
