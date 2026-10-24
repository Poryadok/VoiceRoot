package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
)

func previewTestSnapshot(file uuid.UUID, revision uint64, status string) *eventsv1.FilePreviewSnapshot {
	s := &eventsv1.FilePreviewSnapshot{ProtocolVersion: 1, FileId: file.String(), MetadataRevision: revision, Status: status}
	if status == "ready" {
		s.FileType = "image"
		s.ScanResult = "clean"
		s.SizeBytes = 42
		s.Width = proto.Int32(120)
		s.Height = proto.Int32(80)
		s.HasThumbnail = true
	}
	return s
}

func TestFilePreviewClientIdentityExcludesOnlyOwnerEnrichment(t *testing.T) {
	row := MessageRow{ID: uuid.New(), AdmissionRequestHash: "caller-payload-hash", AdmissionRequestRPC: "SendMessage", AttachmentsJSON: `[{"file_id":"00000000-0000-4000-8000-000000000001","type":"photo","caption":"keep","width":20,"url":"old"}]`, MentionsJSON: "[]"}
	before, err := attachmentIntentIdentity(row, nil)
	require.NoError(t, err)
	row.AttachmentsJSON = `[{"file_id":"00000000-0000-4000-8000-000000000001","type":"photo","caption":"keep","width":99,"processing_status":"deleted"}]`
	after, err := attachmentIntentIdentity(row, nil)
	require.NoError(t, err)
	require.Equal(t, before, after)
	row.AdmissionRequestHash = "changed-caller-payload"
	changed, err := attachmentIntentIdentity(row, nil)
	require.NoError(t, err)
	require.NotEqual(t, before, changed)
}

func TestFilePreviewConcurrentMultiFileInsertUsesOneLockOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	files := []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000001"), uuid.MustParse("00000000-0000-4000-8000-000000000002")}
	for _, file := range files {
		require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(previewTestSnapshot(file, 1, "ready"))))
	}
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback(context.Background()) }()
	_, err = holder.Exec(ctx, `SELECT 1 FROM messaging_file_preview_snapshots WHERE file_id=$1 FOR UPDATE`, files[0])
	require.NoError(t, err)
	results := make(chan error, 2)
	for order := 0; order < 2; order++ {
		go func(reverse int) {
			id := uuid.New()
			row := MessageRow{ID: id, ChatID: uuid.New(), SenderProfileID: uuid.New(), Type: "regular", MentionsJSON: "[]"}
			var attachments []map[string]string
			for slot := 0; slot < 2; slot++ {
				file := files[(slot+reverse)%2]
				ref := &filev1.FileReferenceKey{FileId: file.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: id.String()}
				attachments = append(attachments, map[string]string{"file_id": file.String(), "type": "photo"})
				row.FilePreviewBindings = append(row.FilePreviewBindings, FilePreviewBinding{Slot: slot, FileID: file.String(), Kind: "photo", Reference: ref, Snapshot: previewTestSnapshot(file, 1, "ready")})
			}
			raw, _ := json.Marshal(attachments)
			row.AttachmentsJSON = string(raw)
			_, _, err := s.InsertMessageWithOutbox(ctx, row, nil, nil)
			results <- err
		}(order)
	}
	require.Eventually(t, func() bool {
		var count int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%messaging_file_preview_snapshots%'`).Scan(&count)
		return err == nil && count == 2
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, holder.Commit(ctx))
	require.NoError(t, <-results)
	require.NoError(t, <-results)
}

func TestFilePreviewLegacyRecoveryAndFrozenDestination(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	file, id, chat, space := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO messaging_space_lifecycle_fences(space_id,deletion_operation_id,generation,state,source_schedule_generation,source_manifest_id,source_manifest_sha256,source_manifest_item_count,request_bytes,request_sha256,receipt_bytes) VALUES($1,$2,2,'LIVE',1,$3,$4,0,'',$4,'')`, space, uuid.New(), uuid.New(), make([]byte, 32))
	require.NoError(t, err)
	row := MessageRow{ID: id, ChatID: chat, SenderProfileID: uuid.New(), Type: "regular", ContentType: "photo", AttachmentsJSON: `[{"file_id":"` + file.String() + `","type":"photo","width":999,"url":"stale"}]`, MentionsJSON: "[]", GhostOnly: true}
	_, err = s.InsertMessage(ctx, row)
	require.NoError(t, err)
	rows, err := s.LegacyPreviewRows(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	ref := &filev1.FileReferenceKey{FileId: file.String(), OwnerId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, ScopeSpaceId: proto.String(space.String())}
	request := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, OperationId: uuid.NewSHA1(id, []byte("voice.message.preview.backfill.acquire.v1")).String(), References: []*filev1.FileReferenceKey{ref}}
	require.NoError(t, s.RepairLegacyFilePreviews(ctx, row, &space, request, func(_ context.Context, prepared *MessageRow, _ *filev1.AcquireFileReferencesRequest) error {
		prepared.FilePreviewBindings = []FilePreviewBinding{{Slot: 0, FileID: file.String(), Kind: "photo", Reference: ref, Snapshot: previewTestSnapshot(file, 1, "ready")}}
		return nil
	}))
	rows, err = s.LegacyPreviewRows(ctx)
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = pool.Exec(ctx, `UPDATE messaging_space_lifecycle_fences SET state='FROZEN',generation=1 WHERE space_id=$1`, space)
	require.NoError(t, err)
	require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(previewTestSnapshot(file, 2, "deleted"))))
	work, err := s.FilePreviewPending(ctx)
	require.NoError(t, err)
	require.Len(t, work, 1)
	_, err = s.RefreshFilePreview(ctx, work[0], &space)
	require.ErrorIs(t, err, ErrSpaceLifecycleOrder)
	work, err = s.FilePreviewPending(ctx)
	require.NoError(t, err)
	require.Len(t, work, 1, "frozen is retained, never ACK-and-forgotten")
	_, err = pool.Exec(ctx, `UPDATE messaging_space_lifecycle_fences SET state='LIVE',generation=2 WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = s.RefreshFilePreview(ctx, work[0], &space)
	require.NoError(t, err)
	saved, err := s.GetMessageByID(ctx, id)
	require.NoError(t, err)
	require.NotContains(t, saved.AttachmentsJSON, `"width"`)
	require.Contains(t, saved.AttachmentsJSON, "deleted")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE subject='message.edited'`).Scan(&count))
	require.Zero(t, count, "ghost projection has no public event")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM messaging_attachment_send_intents WHERE message_id=$1`, id).Scan(&state))
	require.Equal(t, "COMMITTED", state)
}
func previewTestEvent(s *eventsv1.FilePreviewSnapshot) *eventsv1.FileProcessed {
	return &eventsv1.FileProcessed{FileId: s.FileId, Status: s.Status, Preview: s}
}

func TestFilePreviewRevisionsMultipleSlotsAndForwardedCopies(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	file := uuid.New()
	chats := []uuid.UUID{uuid.New(), uuid.New()}
	var ids []uuid.UUID
	for _, chat := range chats {
		id := uuid.New()
		ids = append(ids, id)
		ref := &filev1.FileReferenceKey{FileId: file.String(), OwnerId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}
		raw := `[{"type":"photo","file_id":"` + file.String() + `","caption":"keep","url":"https://untrusted"},{"type":"photo","file_id":"` + file.String() + `"}]`
		bindings := []FilePreviewBinding{{Slot: 0, FileID: file.String(), Kind: "photo", Reference: ref, Snapshot: previewTestSnapshot(file, 1, "ready")}, {Slot: 1, FileID: file.String(), Kind: "photo", Reference: ref, Snapshot: previewTestSnapshot(file, 1, "ready")}}
		_, _, err := s.InsertMessageWithOutbox(ctx, MessageRow{ID: id, ChatID: chat, SenderProfileID: uuid.New(), Type: "regular", ContentType: "photo", AttachmentsJSON: raw, MentionsJSON: "[]", FilePreviewBindings: bindings}, nil, nil)
		require.NoError(t, err)
	}
	next := previewTestSnapshot(file, 3, "ready")
	next.Width = nil
	next.HasThumbnail = false
	for _, revision := range []uint64{3, 1, 2, 3} {
		e := proto.Clone(next).(*eventsv1.FilePreviewSnapshot)
		e.MetadataRevision = revision
		require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(e)))
	}
	work, err := s.FilePreviewPending(ctx)
	require.NoError(t, err)
	require.Len(t, work, 2)
	for _, w := range work {
		n, err := s.RefreshFilePreview(ctx, w, nil)
		require.NoError(t, err)
		require.Equal(t, 1, n)
	}
	for _, id := range ids {
		row, err := s.GetMessageByID(ctx, id)
		require.NoError(t, err)
		require.NotContains(t, row.AttachmentsJSON, `"url"`)
		require.NotContains(t, row.AttachmentsJSON, `"width"`)
		require.Contains(t, row.AttachmentsJSON, `"caption": "keep"`)
	}
	conflict := proto.Clone(next).(*eventsv1.FilePreviewSnapshot)
	conflict.Height = proto.Int32(99)
	require.Error(t, s.PersistFilePreview(ctx, previewTestEvent(conflict)))
	require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(previewTestSnapshot(file, 4, "deleted"))))
	require.Error(t, s.PersistFilePreview(ctx, previewTestEvent(previewTestSnapshot(file, 5, "ready"))))
	require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(previewTestSnapshot(file, 3, "ready"))))
	work, err = s.FilePreviewPending(ctx)
	require.NoError(t, err)
	for _, w := range work {
		_, err = s.RefreshFilePreview(ctx, w, nil)
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE subject='message.edited'`).Scan(&count))
	require.Equal(t, 4, count)
	for _, id := range ids {
		row, err := s.GetMessageByID(ctx, id)
		require.NoError(t, err)
		var attachments []map[string]any
		require.NoError(t, json.Unmarshal([]byte(row.AttachmentsJSON), &attachments))
		for _, a := range attachments {
			require.Equal(t, "deleted", a["processing_status"])
			require.NotContains(t, a, "height")
		}
	}
}

func TestFilePreviewBeforeFirstInsertRejectsStaleReady(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	file, id, chat := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(previewTestSnapshot(file, 2, "deleted"))))
	ref := &filev1.FileReferenceKey{FileId: file.String(), OwnerId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}
	row := MessageRow{ID: id, ChatID: chat, SenderProfileID: uuid.New(), Type: "regular", AttachmentsJSON: `[{"file_id":"` + file.String() + `","type":"photo"}]`, MentionsJSON: "[]", FilePreviewBindings: []FilePreviewBinding{{Slot: 0, FileID: file.String(), Kind: "photo", Reference: ref, Snapshot: previewTestSnapshot(file, 1, "ready")}}}
	_, _, err := s.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.Error(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&count))
	require.Zero(t, count)
}
