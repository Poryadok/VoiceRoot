package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"
)

func TestPreviewProducerAtomicReplayAndExactReferenceResolver(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	defer pool.Close()
	applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000004_file_reference_lifecycle.up.sql"))
	applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000005_reference_gc_receipts.up.sql"))
	files := store.NewFilesStore(pool)
	id, owner, space := uuid.New(), uuid.New(), uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "image.png", MimeType: "image/png", SizeBytes: 42, R2Key: "private/original", Status: "pending_upload", FileType: "image", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmUpload(ctx, id, "hash")
	require.NoError(t, err)
	_, err = files.ApplyImageProcessing(ctx, id, "private/converted", "private/thumbnail", 120, 80)
	require.NoError(t, err)
	var first []byte
	published, err := files.DispatchPreviewEvent(ctx, func(_ context.Context, raw []byte) error {
		first = append([]byte(nil), raw...)
		return errors.New("PubAck unavailable")
	})
	require.Error(t, err)
	require.False(t, published)
	published, err = files.DispatchPreviewEvent(ctx, func(_ context.Context, raw []byte) error { require.Equal(t, first, raw); return nil })
	require.NoError(t, err)
	require.True(t, published)
	published, err = files.DispatchPreviewEvent(ctx, func(_ context.Context, raw []byte) error {
		var e eventsv1.FileStreamEvent
		require.NoError(t, proto.Unmarshal(raw, &e))
		require.Equal(t, "ready", e.GetFileProcessed().GetPreview().GetStatus())
		require.NotContains(t, string(raw), "private/")
		return nil
	})
	require.NoError(t, err)
	require.True(t, published)
	_, err = pool.Exec(ctx, `INSERT INTO file_space_lifecycle_fences(space_id,generation,state) VALUES($1,0,'LIVE');`, space)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id,scope_space_id) VALUES($1,1,$2,$3)`, id, owner, space)
	require.NoError(t, err)
	ref := &filev1.FileReferenceKey{FileId: id.String(), OwnerId: owner.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, ScopeSpaceId: proto.String(space.String())}
	request := &filev1.ResolveMessageAttachmentPreviewRequest{ProtocolVersion: 1, Reference: ref}
	service := grpcsvc.New(grpcsvc.Deps{Files: files})
	call := func(issuer string, r *filev1.ResolveMessageAttachmentPreviewRequest) (*filev1.ResolveMessageAttachmentPreviewResponse, error) {
		hash, e := principal.RequestHash(r)
		require.NoError(t, e)
		bound := principal.WithVerified(ctx, principal.Principal{Kind: "service", Subject: "service:" + issuer, Issuer: issuer, Audience: "file", RPC: filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
		return service.ResolveMessageAttachmentPreview(bound, r)
	}
	response, err := call("messaging", request)
	require.NoError(t, err)
	require.True(t, proto.Equal(ref, response.Reference))
	require.EqualValues(t, 3, response.Snapshot.MetadataRevision)
	require.True(t, response.Snapshot.HasThumbnail)
	_, err = call("story", request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	wrong := proto.Clone(request).(*filev1.ResolveMessageAttachmentPreviewRequest)
	wrong.Reference.ScopeSpaceId = nil
	_, err = call("messaging", wrong)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = pool.Exec(ctx, `UPDATE file_space_lifecycle_fences SET state='FROZEN' WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = call("messaging", request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = pool.Exec(ctx, `UPDATE file_space_lifecycle_fences SET state='LIVE' WHERE space_id=$1`, space)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		change func(*filev1.FileReferenceKey)
		want   codes.Code
	}{
		{"foreign owner", func(r *filev1.FileReferenceKey) { r.OwnerId = uuid.NewString() }, codes.FailedPrecondition},
		{"missing fence", func(r *filev1.FileReferenceKey) { r.ScopeSpaceId = proto.String(uuid.NewString()) }, codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := proto.Clone(request).(*filev1.ResolveMessageAttachmentPreviewRequest)
			tc.change(bad.Reference)
			response, err := call("messaging", bad)
			require.Equal(t, tc.want, status.Code(err))
			require.Nil(t, response, "denial cannot return preview metadata")
		})
	}
	var fences int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_space_lifecycle_fences`).Scan(&fences))
	require.Equal(t, 1, fences, "resolver cannot manufacture a missing LIVE fence")
	for _, state := range []string{"FROZEN", "PURGED"} {
		_, err = pool.Exec(ctx, `UPDATE file_space_lifecycle_fences SET state=$1 WHERE space_id=$2`, state, space)
		require.NoError(t, err)
		response, err = call("messaging", request)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		require.Nil(t, response)
	}
	_, err = pool.Exec(ctx, `UPDATE file_space_lifecycle_fences SET state='LIVE' WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, id)
	require.NoError(t, err)
	response, err = call("messaging", request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Nil(t, response, "clock expiry cannot manufacture a revision")
	_, err = pool.Exec(ctx, `UPDATE files SET expires_at=NULL WHERE id=$1`, id)
	require.NoError(t, err)
	require.NoError(t, files.MarkDeleted(ctx, id))
	response, err = call("messaging", request)
	require.NoError(t, err)
	require.Equal(t, "deleted", response.Snapshot.Status)
	require.Empty(t, response.Snapshot.FileType)
	require.Nil(t, response.Snapshot.Width)
	_, err = pool.Exec(ctx, `UPDATE file_references SET released_at=clock_timestamp() WHERE file_id=$1`, id)
	require.NoError(t, err)
	_, err = call("messaging", request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// Exercise the actual owner methods rather than reproducing their SQL in a
// fixture: preview takes blob before reference, as do acquisition and release.
func TestPreviewResolverConcurrentReferenceReleaseAndAcquire(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	defer pool.Close()
	applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000004_file_reference_lifecycle.up.sql"))
	applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000005_reference_gc_receipts.up.sql"))
	files := store.NewFilesStore(pool)
	service := grpcsvc.New(grpcsvc.Deps{Files: files})
	bind := func(ctx context.Context, method string, request proto.Message) context.Context {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		return principal.WithVerified(ctx, principal.Principal{Kind: "service", Subject: "service:messaging", Issuer: "messaging", Audience: "file", RPC: method, RequestID: uuid.NewString(), RequestHash: hash})
	}
	for round := 0; round < 24; round++ {
		id := uuid.New()
		_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "photo.png", MimeType: "image/png", SizeBytes: 42, R2Key: "private/original", Status: "pending_upload", FileType: "image", ScanResult: "pending"})
		require.NoError(t, err)
		_, err = files.ConfirmUpload(ctx, id, "hash")
		require.NoError(t, err)
		_, err = files.ApplyImageProcessing(ctx, id, "private/converted", "private/thumb", 20, 10)
		require.NoError(t, err)
		ref := &filev1.FileReferenceKey{FileId: id.String(), OwnerId: uuid.NewString(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}
		acquire := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{ref}}
		_, err = service.AcquireFileReferences(bind(ctx, filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
		require.NoError(t, err)
		resolve := &filev1.ResolveMessageAttachmentPreviewRequest{ProtocolVersion: 1, Reference: ref}
		release := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{ref}}
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		resolveCtx := bind(bounded, filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, resolve)
		releaseCtx := bind(bounded, filev1.FileService_ReleaseFileReferences_FullMethodName, release)
		start := make(chan struct{})
		resolved, released := make(chan error, 1), make(chan error, 1)
		go func() { <-start; _, e := service.ResolveMessageAttachmentPreview(resolveCtx, resolve); resolved <- e }()
		go func() { <-start; _, e := service.ReleaseFileReferences(releaseCtx, release); released <- e }()
		close(start)
		resolveErr, releaseErr := <-resolved, <-released
		cancel()
		require.NoError(t, releaseErr, "round %d", round)
		if resolveErr != nil {
			require.Equal(t, codes.FailedPrecondition, status.Code(resolveErr), "round %d", round)
		}
		_, err = service.ResolveMessageAttachmentPreview(bind(ctx, filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, resolve), resolve)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		// A different real MESSAGE owner may acquire the still-uncollected blob.
		// Its exact tuple works; the released tuple remains unavailable.
		other := proto.Clone(ref).(*filev1.FileReferenceKey)
		other.OwnerId = uuid.NewString()
		acquire = &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{other}}
		_, err = service.AcquireFileReferences(bind(ctx, filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
		require.NoError(t, err)
		resolve.Reference = other
		response, err := service.ResolveMessageAttachmentPreview(bind(ctx, filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, resolve), resolve)
		require.NoError(t, err)
		require.Equal(t, "ready", response.Snapshot.Status)
	}
}
