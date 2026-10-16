package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
)

func TestMusicRefreshAndTombstoneReachForwardedCopiesAtomically(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	messages := &MessagesStore{Pool: pool}
	file, source, destination := uuid.New(), uuid.New(), uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	for i, chat := range []uuid.UUID{source, destination} {
		_, err := messages.InsertMessage(ctx, MessageRow{ID: ids[i], ChatID: chat, SenderProfileID: uuid.New(), Content: "Music", Type: "regular", ContentType: "music", AttachmentsJSON: `[{"file_id":"` + file.String() + `","type":"music","title":"old"}]`, MentionsJSON: "[]", MusicBinding: &MusicAttachmentBinding{FileID: file, SourceChatID: source, Revision: 1}})
		require.NoError(t, err)
	}
	event := &eventsv1.FileProcessed{FileId: file.String(), Status: "ready", Music: &eventsv1.FileMusicMetadata{ChatId: source.String(), MetadataRevision: 2, Title: "new", SizeBytes: 42, ScanResult: "skipped", DurationSeconds: 7}}
	chats, err := messages.MusicRefreshChats(ctx, event)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{source, destination}, chats)
	for _, chat := range chats {
		n, err := messages.RefreshMusicMetadata(ctx, chat, nil, event)
		require.NoError(t, err)
		require.Equal(t, 1, n)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE subject='message.edited'`).Scan(&count))
	require.Equal(t, 2, count)
	for _, id := range ids {
		row, err := messages.GetMessageByID(ctx, id)
		require.NoError(t, err)
		require.Contains(t, row.AttachmentsJSON, `"title": "new"`)
	}
	event.Status = "deleted"
	event.Music = &eventsv1.FileMusicMetadata{ChatId: source.String(), MetadataRevision: 3}
	for _, chat := range chats {
		n, err := messages.RefreshMusicMetadata(ctx, chat, nil, event)
		require.NoError(t, err)
		require.Equal(t, 1, n)
		n, err = messages.RefreshMusicMetadata(ctx, chat, nil, event)
		require.NoError(t, err)
		require.Zero(t, n)
	}
	for _, id := range ids {
		row, err := messages.GetMessageByID(ctx, id)
		require.NoError(t, err)
		require.NotContains(t, row.AttachmentsJSON, "title")
		require.NotContains(t, row.AttachmentsJSON, "source_chat")
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE subject='message.edited'`).Scan(&count))
	require.Equal(t, 4, count)
}

func TestMusicRefreshRollsBackMetadataWhenOutboxCannotCommit(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	messages := &MessagesStore{Pool: pool}
	file, chat, id := uuid.New(), uuid.New(), uuid.New()
	raw := `[{"file_id":"` + file.String() + `","type":"music","title":"original"}]`
	_, err := messages.InsertMessage(ctx, MessageRow{ID: id, ChatID: chat, SenderProfileID: uuid.New(), Content: "Music", Type: "regular", ContentType: "music", AttachmentsJSON: raw, MentionsJSON: "[]", MusicBinding: &MusicAttachmentBinding{FileID: file, SourceChatID: chat, Revision: 1}})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE message_event_outbox ADD CONSTRAINT reject_test_event CHECK (subject<>'message.edited')`)
	require.NoError(t, err)
	event := &eventsv1.FileProcessed{FileId: file.String(), Status: "ready", Music: &eventsv1.FileMusicMetadata{ChatId: chat.String(), MetadataRevision: 2, Title: "new", SizeBytes: 42, ScanResult: "clean", DurationSeconds: 7}}
	_, err = messages.RefreshMusicMetadata(ctx, chat, nil, event)
	require.Error(t, err)
	row, err := messages.GetMessageByID(ctx, id)
	require.NoError(t, err)
	require.JSONEq(t, raw, row.AttachmentsJSON)
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT music_metadata_revision FROM messages WHERE id=$1`, id).Scan(&revision))
	require.EqualValues(t, 1, revision)
}
