package store

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

func TestSpaceLifecycleFenceReceiptPersistsReplaysAndRestores(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	lifecycle := &SpaceLifecycleStore{Pool: pool}
	spaceID, operationID := uuid.New(), uuid.New()
	prepared, err := lifecycle.PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), ScheduleGeneration: 1,
	})
	require.NoError(t, err)
	globalManifest := proto.Clone(prepared.GetReceipt().GetChatManifest()).(*commonv1.ManifestBinding)

	frozen := &chatv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: globalManifest,
	}}
	first, err := lifecycle.ApplySpaceLifecycleFence(ctx, frozen)
	require.NoError(t, err)
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_CHAT, first.GetReceipt().GetParticipantId())
	require.Equal(t, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, first.GetReceipt().GetAppliedState())
	require.Len(t, first.GetReceipt().GetRequestSha256(), 32)
	requestWire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(frozen)
	require.NoError(t, err)
	expectedHash := sha256.Sum256(append([]byte("voice.chat.v1.ApplySpaceLifecycleFenceRequest\x00"), requestWire...))
	require.Equal(t, expectedHash[:], first.GetReceipt().GetRequestSha256(), "receipt must match Space's typed participant request binding")
	page, err := lifecycle.GetSpacePurgeManifestPage(ctx, &chatv1.GetSpacePurgeManifestPageRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1, ManifestId: globalManifest.ManifestId,
	})
	require.NoError(t, err, "the final Space manifest selects the saved Chat source page")
	require.True(t, proto.Equal(prepared.GetReceipt().GetChatManifest(), page.GetPage().GetManifest()))

	replay, err := lifecycle.ApplySpaceLifecycleFence(ctx, frozen)
	require.NoError(t, err)
	require.True(t, proto.Equal(first, replay))

	changed := proto.Clone(frozen).(*chatv1.ApplySpaceLifecycleFenceRequest)
	changed.Fence = &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:     &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: globalManifest.GetManifestSha256()},
	}
	_, err = lifecycle.ApplySpaceLifecycleFence(ctx, changed)
	require.ErrorIs(t, err, ErrSpaceLifecycleConflict)

	restored, err := lifecycle.ApplySpaceLifecycleFence(ctx, &chatv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 2,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, Manifest: globalManifest,
	}})
	require.NoError(t, err)
	require.Equal(t, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, restored.GetReceipt().GetAppliedState())

	var state string
	var generation int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,generation FROM chat_space_lifecycle_fences WHERE space_id=$1`, spaceID).Scan(&state, &generation))
	require.Equal(t, "LIVE", state)
	require.EqualValues(t, 2, generation)
}
