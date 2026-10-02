package grpcsvc

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"
)

func TestSearchSourceRootBindingsSurviveRestartAndGatePurge(t *testing.T) {
	for _, empty := range []bool{true, false} {
		name := "one chat"
		if empty {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			fixture := startR23SearchFixture(t)
			ids := []uuid.UUID{fixture.targetChat}
			if empty {
				ids = nil
			}
			source := newR23ManifestClientForIDs(t, ids)
			source.page.Manifest.ManifestId = uuid.NewString()
			fixture.first.ChatManifest = source
			request := r23SearchFenceRequest(t, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
			request.Fence.Manifest = &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: bytes.Repeat([]byte{7}, 32), ItemCount: 9}
			frozen, err := fixture.first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, request, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), request)
			require.NoError(t, err)
			require.Equal(t, request.Fence.Manifest.ManifestSha256, frozen.Receipt.ManifestSha256)
			var sourceID, rootID string
			var count int
			require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT manifest_id,root_manifest_id,item_count FROM search_space_chat_manifests WHERE space_id=$1`, r23SearchSpaceID).Scan(&sourceID, &rootID, &count))
			require.Equal(t, source.page.Manifest.ManifestId, sourceID)
			require.Equal(t, request.Fence.Manifest.ManifestId, rootID)
			require.Equal(t, len(ids), count)
			down, err := fixture.pool.Begin(fixture.ctx)
			require.NoError(t, err)
			_, err = down.Exec(fixture.ctx, r23SearchMigrationSQL(t, "000010_chat_manifest_root_binding.down.sql"))
			require.Error(t, err, "retained source/root evidence prevents destructive rollback")
			var pgError *pgconn.PgError
			require.ErrorAs(t, err, &pgError)
			require.Equal(t, "55000", pgError.Code)
			require.NoError(t, down.Rollback(fixture.ctx))
			decide := proto.Clone(request).(*searchv1.ApplySpaceLifecycleFenceRequest)
			decide.Fence.Generation = 2
			decide.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
			changed := proto.Clone(decide).(*searchv1.ApplySpaceLifecycleFenceRequest)
			changed.Fence.Manifest.ManifestSha256[0] ^= 1
			restarted := fixture.newServer()
			_, err = restarted.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, changed, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), changed)
			require.Equal(t, codes.FailedPrecondition, status.Code(err), "complete source evidence must also bind the exact Space root")
			_, err = restarted.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, decide, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), decide)
			require.NoError(t, err)
			purge := r23SearchPurgeRequest()
			purge.Purge.Manifest = proto.Clone(request.Fence.Manifest).(*commonv1.ManifestBinding)
			completed, err := restarted.PurgeSpace(r23SearchTrustedSpaceContext(t, purge, searchv1.SearchService_PurgeSpace_FullMethodName), purge)
			require.NoError(t, err)
			replay, err := fixture.newServer().PurgeSpace(r23SearchTrustedSpaceContext(t, purge, searchv1.SearchService_PurgeSpace_FullMethodName), purge)
			require.NoError(t, err)
			require.True(t, proto.Equal(completed, replay))
			var controls int
			require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM message_search_documents WHERE chat_id=$1`, fixture.controlChat).Scan(&controls))
			require.Positive(t, controls, "another Space's message index survives")
		})
	}
}
