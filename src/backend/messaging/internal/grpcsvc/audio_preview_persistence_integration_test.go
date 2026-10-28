package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/store"
)

func TestAudioPreviewInsertionAndRetryWithExactReference(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyBaseMessagingMigrations(t, ctx, pool)
	fileID, messageID, clientID := uuid.NewString(), uuid.New(), uuid.New()
	row := store.MessageRow{ID: messageID, ChatID: uuid.New(), SenderProfileID: uuid.New(), Type: "regular", ContentType: "voice", ClientMessageID: &clientID, AttachmentsJSON: `[{"file_id":"` + fileID + `","type":"voice_message"}]`, MentionsJSON: "[]"}
	ref := &filev1.FileReferenceKey{FileId: fileID, OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}
	request := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{ref}}
	stub := &audioPreviewStub{responses: map[string]*filev1.ResolveMessageAttachmentPreviewResponse{}}
	coordinator := &AttachmentReferenceCoordinator{Files: stub, Issuer: testSpaceFileProducer(t).Issuer}
	acquired := 0
	acquire := func(_ context.Context, r *filev1.AcquireFileReferencesRequest) error {
		acquired++
		require.Equal(t, messageID.String(), r.References[0].OwnerId)
		stub.responses[fileID] = &filev1.ResolveMessageAttachmentPreviewResponse{Reference: r.References[0], AudioNoGenericPreview: &filev1.AudioNoGenericPreview{ProtocolVersion: 1, FileId: fileID, Status: "ready"}}
		return nil
	}
	messages := &store.MessagesStore{Pool: pool}
	first, err := messages.InsertMessageWithPreparedReferencesAndOutbox(ctx, row, nil, request, acquire, coordinator.PreparePreview, nil)
	require.NoError(t, err)
	second, err := messages.InsertMessageWithPreparedReferencesAndOutbox(ctx, row, nil, request, acquire, coordinator.PreparePreview, nil)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, 1, acquired)
	var count int
	var bindings string
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE chat_id=$1`, row.ChatID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT file_preview_bindings::text FROM messages WHERE id=$1`, messageID).Scan(&bindings))
	require.Equal(t, "[]", bindings)
}
