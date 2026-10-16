package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
)

func TestMusicTombstoneBeforeFirstMessageDoesNotPersistStaleTags(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	messages := &MessagesStore{Pool: pool}
	file, chat := uuid.New(), uuid.New()
	tombstone := &eventsv1.FileProcessed{FileId: file.String(), Status: "deleted", Music: &eventsv1.FileMusicMetadata{ChatId: chat.String(), MetadataRevision: 2}}
	latest, err := messages.PersistMusicSnapshot(ctx, tombstone)
	require.NoError(t, err)
	chats, err := messages.MusicRefreshChats(ctx, latest)
	require.NoError(t, err)
	require.Empty(t, chats)

	row := MessageRow{
		ID: uuid.New(), ChatID: chat, SenderProfileID: uuid.New(), Content: "Music", Type: "regular",
		ContentType: "music", AttachmentsJSON: `[{"file_id":"` + file.String() + `","type":"music","title":"stale title","artist":"stale artist","duration_seconds":7}]`,
		MentionsJSON: "[]", MusicBinding: &MusicAttachmentBinding{
			FileID: file, SourceChatID: chat, Revision: 1, Status: "ready", SizeBytes: 42,
			ScanResult: "clean", Duration: 7, Title: "stale title", Artist: "stale artist",
		},
	}
	request := &filev1.AcquireFileReferencesRequest{
		ProtocolVersion: 1,
		ProducerId:      filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		References: []*filev1.FileReferenceKey{{
			FileId: file.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE,
		}},
	}
	saved, err := messages.InsertMessageWithReferences(ctx, row, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error { return nil })
	require.NoError(t, err)
	require.JSONEq(t, `[{"file_id":"`+file.String()+`","type":"music"}]`, saved.AttachmentsJSON)
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT music_metadata_revision FROM messages WHERE id=$1`, row.ID).Scan(&revision))
	require.EqualValues(t, 2, revision)
}

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
