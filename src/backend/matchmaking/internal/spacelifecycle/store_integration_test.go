package spacelifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/store"
)

func TestSpaceLifecycleFenceReceiptsRestoreAndPurgeSurviveExactReplay(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	gameStore := &store.GameStore{Pool: pool}
	games, err := gameStore.List(ctx, store.ListGamesParams{PageSize: 1, Status: store.StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, games.Games)
	spaceID := uuid.New()
	session, err := (&store.SessionStore{Pool: pool}).Create(ctx, store.CreateSessionParams{
		ProfileID: uuid.New(), GameID: games.Games[0].ID, Mode: "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: time.Now().UTC().Add(time.Hour), SpaceID: &spaceID,
	})
	require.NoError(t, err)

	participant := New(pool)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32), ItemCount: 0}
	operationID := uuid.NewString()
	frozen := &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID,
		Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest: manifest,
	}
	queueCleanups := 0
	cleanupQueue := func(context.Context, uuid.UUID) error { queueCleanups++; return nil }
	freezeReceipt, err := participant.ApplyFence(ctx, frozen, cleanupQueue)
	require.NoError(t, err)
	require.Equal(t, participantRequestDigest(t, &matchmakingv1.ApplySpaceLifecycleFenceRequest{Fence: frozen}), freezeReceipt.GetRequestSha256(), "Space must accept the completed Matchmaking fence receipt")
	replayedFreeze, err := participant.ApplyFence(ctx, frozen, cleanupQueue)
	require.NoError(t, err)
	require.True(t, proto.Equal(freezeReceipt, replayedFreeze))
	require.Equal(t, 2, queueCleanups, "exact replay re-applies idempotent Redis cleanup")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM search_sessions WHERE id=$1`, session.ID).Scan(&state))
	require.Equal(t, store.SessionStatusCancelled, state)

	changed := proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest)
	changed.Manifest.ItemCount++
	_, err = participant.ApplyFence(ctx, changed, cleanupQueue)
	require.ErrorIs(t, err, ErrConflict)

	restored := proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest)
	restored.Generation = 2
	restored.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	_, err = participant.ApplyFence(ctx, restored, cleanupQueue)
	require.NoError(t, err)

	secondOperationID := uuid.NewString()
	secondManifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: bytes.Repeat([]byte{0x5a}, 32), ItemCount: 2}
	secondFreeze := proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest)
	secondFreeze.Generation = 3
	secondFreeze.DeletionOperationId = secondOperationID
	secondFreeze.Manifest = secondManifest
	_, err = participant.ApplyFence(ctx, secondFreeze, cleanupQueue)
	require.NoError(t, err)
	purgeDecided := proto.Clone(secondFreeze).(*commonv1.SpaceLifecycleFenceRequest)
	purgeDecided.Generation = 4
	purgeDecided.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = participant.ApplyFence(ctx, purgeDecided, cleanupQueue)
	require.NoError(t, err)
	purge := &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: secondOperationID,
		Generation: 4, PurgeDecidedAt: timestamppb.Now(),
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING, Manifest: secondManifest,
	}
	purgeCleanups := 0
	cleanupPurge := func(context.Context, uuid.UUID) error { purgeCleanups++; return nil }
	purgeReceipt, err := participant.PurgeSpace(ctx, purge, cleanupPurge)
	require.NoError(t, err)
	require.Equal(t, participantRequestDigest(t, &matchmakingv1.PurgeSpaceRequest{Purge: purge}), purgeReceipt.GetRequestSha256(), "Space must accept the completed Matchmaking purge receipt")
	replayedPurge, err := participant.PurgeSpace(ctx, purge, cleanupPurge)
	require.NoError(t, err)
	require.True(t, proto.Equal(purgeReceipt, replayedPurge))
	require.Equal(t, 2, purgeCleanups)
	var sessions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE space_id=$1`, spaceID).Scan(&sessions))
	require.Zero(t, sessions)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM matchmaking_space_lifecycle_fence_heads WHERE space_id=$1`, spaceID).Scan(&state))
	require.Equal(t, "PURGED", state)
}

func participantRequestDigest(t *testing.T, request proto.Message) []byte {
	t.Helper()
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	digest := sha256.Sum256(append(append([]byte(request.ProtoReflect().Descriptor().FullName()), 0), wire...))
	return digest[:]
}
