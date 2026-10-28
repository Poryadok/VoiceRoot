package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
)

func TestAudioLegacyEmptyBindingsRemainFencedAndIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	file, id, chat, space := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO messaging_space_lifecycle_fences(space_id,deletion_operation_id,generation,state,source_schedule_generation,source_manifest_id,source_manifest_sha256,source_manifest_item_count,request_bytes,request_sha256,receipt_bytes) VALUES($1,$2,2,'LIVE',1,$3,$4,0,'',$4,'')`, space, uuid.New(), uuid.New(), make([]byte, 32))
	require.NoError(t, err)
	row := MessageRow{ID: id, ChatID: chat, SenderProfileID: uuid.New(), Type: "regular", ContentType: "voice", AttachmentsJSON: `[{"file_id":"` + file.String() + `","type":"voice_message"}]`, MentionsJSON: "[]"}
	_, err = s.InsertMessage(ctx, row)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messaging_space_lifecycle_fences SET state='FROZEN',generation=1 WHERE space_id=$1`, space)
	require.NoError(t, err)
	require.NoError(t, s.RecordLegacyPreviewError(ctx, id, "reference unavailable"))
	ref := &filev1.FileReferenceKey{FileId: file.String(), OwnerId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, ScopeSpaceId: proto.String(space.String())}
	req := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, OperationId: uuid.NewSHA1(id, []byte("voice.message.preview.backfill.acquire.v1")).String(), References: []*filev1.FileReferenceKey{ref}}
	prepare := func(_ context.Context, prepared *MessageRow, _ *filev1.AcquireFileReferencesRequest) error {
		prepared.FilePreviewBindings = []FilePreviewBinding{}
		return nil
	}
	require.Error(t, s.RepairLegacyFilePreviews(ctx, row, &space, req, prepare))
	var errorsBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_file_preview_errors WHERE message_id=$1`, id).Scan(&errorsBefore))
	require.Equal(t, 1, errorsBefore)
	_, err = pool.Exec(ctx, `UPDATE messaging_space_lifecycle_fences SET state='LIVE',generation=2 WHERE space_id=$1`, space)
	require.NoError(t, err)
	require.NoError(t, s.RepairLegacyFilePreviews(ctx, row, &space, req, prepare))
	row.FilePreviewBindings = []FilePreviewBinding{}
	require.NoError(t, s.BindLegacyFilePreviews(ctx, row, &space))
	var bindings, attachments string
	var errorsAfter, edits int
	require.NoError(t, pool.QueryRow(ctx, `SELECT file_preview_bindings::text,attachments::text FROM messages WHERE id=$1`, id).Scan(&bindings, &attachments))
	require.Equal(t, "[]", bindings)
	require.JSONEq(t, row.AttachmentsJSON, attachments)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_file_preview_errors WHERE message_id=$1`, id).Scan(&errorsAfter))
	require.Zero(t, errorsAfter)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE subject='message.edited'`).Scan(&edits))
	require.Zero(t, edits)
}
