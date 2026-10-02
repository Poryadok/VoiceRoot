package spacelifecycle

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	"voice.app/voice/common/v1"
	"voice/backend/pkg/integrationtest"
)

func TestPostgresSpaceLifecycleFenceAndPurgeReceipts(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceLifecyclePostgres(t, ctx)
	store := NewPostgresStore(pool)
	require.NoError(t, store.CheckSchema(ctx))
	spaceID, operationID := uuid.NewString(), uuid.NewString()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32), ItemCount: 3}
	frozen := fenceRequest(spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	callbacks := 0
	failedEffect := errors.New("injected ejection failure")
	_, err := store.ApplyFence(ctx, frozen, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error {
		callbacks++
		return failedEffect
	})
	require.ErrorIs(t, err, failedEffect)
	require.ErrorIs(t, store.CheckAdmission(ctx, spaceID), ErrSpaceFrozen, "the durable fence must deny admission before effects finish")

	var pendingReceipt []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT receipt_bytes FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=1`, uuid.MustParse(spaceID)).Scan(&pendingReceipt))
	require.Empty(t, pendingReceipt, "failed effects must not publish a completed receipt")

	frozenReceipt, err := store.ApplyFence(ctx, proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest), func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error {
		callbacks++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, callbacks, "an exact pending retry reruns idempotent effects")
	require.Equal(t, participantRequestDigest(t, &callsv1.ApplySpaceLifecycleFenceRequest{Fence: frozen}), frozenReceipt.GetRequestSha256(), "Space must accept the completed Voice fence receipt")
	frozenReplay, err := NewPostgresStore(pool).ApplyFence(ctx, proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest), func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error {
		t.Fatal("completed fence replay must not repeat effects")
		return nil
	})
	require.NoError(t, err)
	require.True(t, proto.Equal(frozenReceipt, frozenReplay))

	changed := proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest)
	changed.Manifest.ItemCount++
	_, err = store.ApplyFence(ctx, changed, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return nil })
	require.ErrorIs(t, err, ErrConflict, "same generation with changed bytes must conflict")

	live := fenceRequest(spaceID, operationID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, manifest)
	_, err = store.ApplyFence(ctx, live, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return nil })
	require.NoError(t, err)
	require.NoError(t, store.CheckAdmission(ctx, spaceID), "LIVE releases Voice room admission")

	refrozen := fenceRequest(spaceID, operationID, 3, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	_, err = store.ApplyFence(ctx, refrozen, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return nil })
	require.NoError(t, err)
	decided := fenceRequest(spaceID, operationID, 4, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, manifest)
	_, err = store.ApplyFence(ctx, decided, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return nil })
	require.NoError(t, err)
	purge := &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID, DeletionOperationId: operationID, Generation: 4,
		PurgeDecidedAt: timestamppb.New(time.Now().UTC().Add(-time.Minute)), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
		Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding),
	}
	purgeEffects := 0
	completed, err := store.PurgeSpace(ctx, purge, func(context.Context, *commonv1.SpacePurgeRequest) error {
		purgeEffects++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, purgeEffects)
	require.Equal(t, participantRequestDigest(t, &callsv1.PurgeSpaceRequest{Purge: purge}), completed.GetRequestSha256(), "Space must accept the completed Voice purge receipt")
	require.True(t, completed.GetCompletedAt().AsTime().After(purge.GetPurgeDecidedAt().AsTime()), "completion time must reflect when Voice finished the purge")

	replay, err := NewPostgresStore(pool).PurgeSpace(ctx, proto.Clone(purge).(*commonv1.SpacePurgeRequest), func(context.Context, *commonv1.SpacePurgeRequest) error {
		t.Fatal("completed purge replay must not repeat effects")
		return nil
	})
	require.NoError(t, err)
	require.True(t, proto.Equal(completed, replay))

	_, err = pool.Exec(ctx, `UPDATE voice_space_lifecycle_purge_receipts SET receipt_sha256=$2 WHERE space_id=$1 AND generation=$3`, uuid.MustParse(spaceID), make([]byte, 32), int64(4))
	require.NoError(t, err)
	_, err = store.PurgeSpace(ctx, proto.Clone(purge).(*commonv1.SpacePurgeRequest), func(context.Context, *commonv1.SpacePurgeRequest) error { return nil })
	require.ErrorIs(t, err, ErrUnavailable, "stored receipt bytes must be checked against their persisted digest")
}

func participantRequestDigest(t *testing.T, request proto.Message) []byte {
	t.Helper()
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	digest := sha256.Sum256(append(append([]byte(request.ProtoReflect().Descriptor().FullName()), 0), wire...))
	return digest[:]
}

func TestPostgresSpaceLifecyclePurgeRequiresDecisionFence(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceLifecyclePostgres(t, ctx)
	store := NewPostgresStore(pool)
	spaceID, operationID := uuid.NewString(), uuid.NewString()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	frozen := fenceRequest(spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	_, err := store.ApplyFence(ctx, frozen, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return nil })
	require.NoError(t, err)
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
	pool := integrationtest.StartPostgres(t, ctx, "voice_space_lifecycle", filepath.Join(migrations, "000001_room_lifecycle.up.sql"))
	for _, name := range []string{"000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms", "000005_game_session_close", "000006_game_session_roster_lease", "000007_t17_sdk_conversion_fence", "000008_account_voice_fence", "000009_space_lifecycle"} {
		body, err := os.ReadFile(filepath.Join(migrations, name+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}
