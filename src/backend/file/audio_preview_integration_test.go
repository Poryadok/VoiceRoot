package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"
)

func TestAudioPreviewRequiresExactLiveReference(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	defer pool.Close()
	applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000014_audio_intent.up.sql"))
	files := store.NewFilesStore(pool)
	id, owner := uuid.New(), uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "voice.wav", MimeType: "audio/wav", SizeBytes: 42, R2Key: "private/audio", Status: "pending_upload", FileType: "audio", ScanResult: "pending", UploadIntent: "audio"})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET status='ready', scan_result='clean' WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id) VALUES($1,1,$2)`, id, owner)
	require.NoError(t, err)
	ref := &filev1.FileReferenceKey{FileId: id.String(), OwnerId: owner.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}
	request := &filev1.ResolveMessageAttachmentPreviewRequest{ProtocolVersion: 1, Reference: ref}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	bound := principal.WithVerified(ctx, principal.Principal{Kind: "service", Subject: "service:messaging", Issuer: "messaging", Audience: "file", RPC: filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	service := grpcsvc.New(grpcsvc.Deps{Files: files})
	response, err := service.ResolveMessageAttachmentPreview(bound, request)
	require.NoError(t, err)
	require.Nil(t, response.GetSnapshot())
	require.Equal(t, ref.FileId, response.GetReference().GetFileId())
	require.EqualValues(t, 1, response.GetAudioNoGenericPreview().GetProtocolVersion())
	require.Equal(t, id.String(), response.GetAudioNoGenericPreview().GetFileId())
	require.Equal(t, "ready", response.GetAudioNoGenericPreview().GetStatus())
	_, err = pool.Exec(ctx, `UPDATE files SET scan_result='pending' WHERE id=$1`, id)
	require.NoError(t, err)
	response, err = service.ResolveMessageAttachmentPreview(bound, request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Nil(t, response)
	_, err = pool.Exec(ctx, `UPDATE files SET scan_result='clean' WHERE id=$1`, id)
	require.NoError(t, err)
	for _, tc := range []struct {
		name  string
		query string
	}{
		{"wrong type", `UPDATE files SET file_type='image' WHERE id=$1`},
		{"oversize", `UPDATE files SET size_bytes=10485761 WHERE id=$1`},
		{"encrypted", `UPDATE files SET is_e2e=true WHERE id=$1`},
		{"music", `UPDATE files SET upload_intent='music' WHERE id=$1`},
		{"sticker", `UPDATE files SET upload_intent='sticker' WHERE id=$1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, e := pool.Exec(ctx, tc.query, id)
			require.NoError(t, e)
			result, e := service.ResolveMessageAttachmentPreview(bound, request)
			require.Equal(t, codes.FailedPrecondition, status.Code(e))
			require.Nil(t, result)
			_, e = pool.Exec(ctx, `UPDATE files SET file_type='audio',size_bytes=42,is_e2e=false,upload_intent='audio' WHERE id=$1`, id)
			require.NoError(t, e)
		})
	}
	_, err = pool.Exec(ctx, `UPDATE files SET status='processing' WHERE id=$1`, id)
	require.NoError(t, err)
	response, err = service.ResolveMessageAttachmentPreview(bound, request)
	require.NoError(t, err)
	require.Equal(t, "processing", response.GetAudioNoGenericPreview().GetStatus())
	_, err = pool.Exec(ctx, `UPDATE files SET status='ready',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, id)
	require.NoError(t, err)
	response, err = service.ResolveMessageAttachmentPreview(bound, request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Nil(t, response)
	_, err = pool.Exec(ctx, `UPDATE files SET expires_at=NULL WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET upload_intent='' WHERE id=$1`, id)
	require.NoError(t, err)
	response, err = service.ResolveMessageAttachmentPreview(bound, request)
	require.NoError(t, err)
	require.Nil(t, response.GetAudioNoGenericPreview())
	require.Equal(t, "audio", response.GetSnapshot().GetFileType())
	_, err = pool.Exec(ctx, `UPDATE files SET upload_intent='audio' WHERE id=$1`, id)
	require.NoError(t, err)
	space := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO file_space_lifecycle_fences(space_id,generation,state) VALUES($1,0,'FROZEN')`, space)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE file_references SET scope_space_id=$1 WHERE file_id=$2`, space, id)
	require.NoError(t, err)
	ref.ScopeSpaceId = proto.String(space.String())
	hash, err = principal.RequestHash(request)
	require.NoError(t, err)
	bound = principal.WithVerified(ctx, principal.Principal{Kind: "service", Subject: "service:messaging", Issuer: "messaging", Audience: "file", RPC: filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	response, err = service.ResolveMessageAttachmentPreview(bound, request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Nil(t, response)
	_, err = pool.Exec(ctx, `UPDATE file_space_lifecycle_fences SET state='LIVE' WHERE space_id=$1`, space)
	require.NoError(t, err)
	response, err = service.ResolveMessageAttachmentPreview(bound, request)
	require.NoError(t, err)
	require.Equal(t, "ready", response.GetAudioNoGenericPreview().GetStatus())
	_, err = pool.Exec(ctx, `UPDATE file_references SET released_at=clock_timestamp() WHERE file_id=$1`, id)
	require.NoError(t, err)
	response, err = service.ResolveMessageAttachmentPreview(bound, request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Nil(t, response)
}
