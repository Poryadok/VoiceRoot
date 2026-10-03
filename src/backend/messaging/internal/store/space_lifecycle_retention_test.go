package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

func TestRestoredSpaceProducerEvidenceExpiresAtDatabaseReplayBoundary(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	space, operation := uuid.New(), uuid.New()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: messagingChatManifestSHA(space, operation, 7, nil)}
	_, err := s.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{SpaceID: space, DeletionOperationID: operation, ScheduleGeneration: 7, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, PageSha256: make([]byte, 32)}, SealsManifest: true, RequestBytes: []byte("page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("page-receipt")})
	require.NoError(t, err)
	fence := SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: operation, Generation: 7, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest, RequestBytes: []byte("frozen"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("frozen-receipt")}
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	_, err = s.SpaceFileProducerReferences(ctx, space, operation, 7)
	require.NoError(t, err)
	fence.Generation = 8
	fence.State = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	fence.RequestBytes = []byte("live")
	fence.ReceiptBytes = []byte("live-receipt")
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	require.NoError(t, s.CleanupSpaceLifecycleEvidence(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_space_file_producers WHERE space_id=$1`, space).Scan(&count))
	require.Equal(t, 1, count)
	_, err = pool.Exec(ctx, `UPDATE messaging_space_lifecycle_operations SET retain_until=clock_timestamp()-interval '1 second' WHERE space_id=$1 AND generation=8`, space)
	require.NoError(t, err)
	require.NoError(t, s.CleanupSpaceLifecycleEvidence(ctx))
	require.NoError(t, s.CleanupSpaceLifecycleEvidence(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_space_file_producers WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count)
	_, err = s.SpaceFileProducerReferences(ctx, space, operation, 7)
	require.Error(t, err, "restored fence cannot recapture an expired frozen snapshot")
}
