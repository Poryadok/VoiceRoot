package grpcsvc

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/authctx"
)

func TestMigratedReferenceCannotBypassFrozenFenceThroughLegacyUploaderReads(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	service := r23FileServer(pool)
	service.referenceAuthorityActive = false
	service.referenceLifecycleEnabled = true
	file := insertR23ReadyFile(t, ctx, pool, "migrated-freeze")
	var uploader uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT uploader_profile_id FROM files WHERE id=$1`, file).Scan(&uploader))
	space := uuid.New()
	ref := r23MessageReference(file, uuid.New(), space)
	acquire := r23AcquireRequest(ref)
	_, err := service.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	profile := metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderProfileID, uploader.String()))
	_, err = service.GetFileMetadata(profile, &filev1.GetFileMetadataRequest{FileId: file.String()})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE file_space_lifecycle_fences SET state='FROZEN' WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = service.GetFileMetadata(profile, &filev1.GetFileMetadataRequest{FileId: file.String()})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = service.GetBulkMetadata(profile, &filev1.GetBulkMetadataRequest{FileIds: []string{file.String()}})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "bulk denies whole result")
}
