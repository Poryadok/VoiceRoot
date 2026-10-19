package spacelifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"voice.app/voice/common/v1"
	"voice/backend/voice/internal/spacegatetest"
)

func TestPostgresSpaceLifecyclePendingReplayRetainsBinding(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceLifecyclePostgres(t, ctx)
	s := NewPostgresStore(pool)
	require.NoError(t, s.CheckSchema(ctx))
	space, operation := uuid.NewString(), uuid.NewString()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	frozen := fenceRequest(space, operation, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	failure := errors.New("ejection unavailable")
	_, err := s.ApplyFence(ctx, frozen, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return failure })
	require.ErrorIs(t, err, failure)
	callbacks := 0
	for range 2 {
		receipt, err := s.ApplyFence(ctx, frozen, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { callbacks++; return nil })
		require.ErrorIs(t, err, ErrUnavailable)
		require.Nil(t, receipt)
	}
	require.Equal(t, 2, callbacks)
	changed := proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest)
	changed.Manifest.ItemCount++
	_, err = s.ApplyFence(ctx, changed, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error {
		t.Fatal("changed body must not run effects")
		return nil
	})
	require.ErrorIs(t, err, ErrConflict)
	require.ErrorIs(t, s.CheckAdmission(ctx, space), ErrSpaceFrozen)
	live := fenceRequest(space, operation, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, manifest)
	_, err = s.ApplyFence(ctx, live, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error {
		t.Fatal("pending freeze cannot authorize LIVE")
		return nil
	})
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestPostgresSpaceLifecyclePurgeRequiresDecisionFence(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceLifecyclePostgres(t, ctx)
	store := NewPostgresStore(pool)
	spaceID, operationID := uuid.NewString(), uuid.NewString()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	frozen := fenceRequest(spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	_, err := store.ApplyFence(ctx, frozen, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return nil })
	require.ErrorIs(t, err, ErrUnavailable)
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: spaceID, DeletionOperationId: operationID, Generation: 1,
		PurgeDecidedAt: timestamppb.Now(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE, Manifest: manifest}
	effects := 0
	_, err = store.PurgeSpace(ctx, purge, func(context.Context, *commonv1.SpacePurgeRequest) error { effects++; return nil })
	require.ErrorIs(t, err, ErrConflict)
	require.Zero(t, effects, "purge effects must not run before the irreversible decision fence")
}

func fenceRequest(spaceID, operationID string, generation uint64, state commonv1.LifecycleFenceState, manifest *commonv1.ManifestBinding) *commonv1.SpaceLifecycleFenceRequest {
	return &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: spaceID, DeletionOperationId: operationID,
		Generation: generation, DesiredState: state, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding)}
}

func startSpaceLifecyclePostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	migrations := filepath.Join(root, "src", "backend", "migrations", "voice_db")
	pool := spacegatetest.StartPostgres(t, ctx, "voice_space_lifecycle", filepath.Join(migrations, "000001_room_lifecycle.up.sql"))
	for _, name := range []string{"000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms", "000005_game_session_close", "000006_game_session_roster_lease", "000007_t17_sdk_conversion_fence", "000008_account_voice_fence", "000009_space_lifecycle", "000010_space_media_admission", "000014_space_media_p3"} {
		body, err := os.ReadFile(filepath.Join(migrations, name+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}
