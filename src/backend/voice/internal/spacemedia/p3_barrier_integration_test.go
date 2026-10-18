package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/voice/internal/spacelifecycle"
)

func TestP3FreezeSerializesBeforeEveryForwardNativeWriter(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	for _, stage := range []string{"claim", "prepare", "fence", "commit", "projection", "repair"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			s := NewPostgresAdmissionStore(pool)
			a := admissionFixture()
			head, err := s.ClaimRoomHead(ctx, a.VoiceRoomID, a.SpaceID.String(), a.RoomID, a.OperationID, true)
			require.NoError(t, err)
			a.RoomGeneration = head.RoomGeneration
			if stage != "claim" && stage != "prepare" {
				require.NoError(t, s.Prepare(ctx, a))
			}
			if stage == "commit" || stage == "projection" || stage == "repair" {
				require.NoError(t, s.Fence(ctx, a))
			}
			if stage == "projection" || stage == "repair" {
				require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
			}
			blocker, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback(context.Background()) }()
			_, err = blocker.Exec(ctx, `SELECT 1 FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, a.VoiceRoomID, int64(a.RoomGeneration))
			require.NoError(t, err)
			request := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: a.SpaceID.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}
			freezeResult := make(chan error, 1)
			go func() {
				_, err := spacelifecycle.NewPostgresStore(pool).ApplyFence(ctx, request, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return nil })
				freezeResult <- err
			}()
			waitForPostgresLockWait(t, ctx, pool, "UPDATE voice_space_media_room_heads SET state='CLOSING'")
			writerResult := make(chan error, 1)
			go func() {
				switch stage {
				case "claim":
					_, err := NewPostgresAdmissionStore(pool).ClaimRoomHead(ctx, uuid.NewString(), a.SpaceID.String(), uuid.NewString(), uuid.New(), true)
					writerResult <- err
				case "prepare":
					writerResult <- NewPostgresAdmissionStore(pool).Prepare(ctx, a)
				case "fence":
					writerResult <- NewPostgresAdmissionStore(pool).Fence(ctx, a)
				case "commit":
					writerResult <- NewPostgresAdmissionStore(pool).MarkCommitted(ctx, a.OperationID, a.Generation)
				case "projection":
					writerResult <- NewPostgresAdmissionStore(pool).MarkProjectionApplied(ctx, a.OperationID, a.Generation)
				case "repair":
					_, err := NewPostgresAdmissionStore(pool).ProjectionGeneration(ctx, a.OperationID, a.Generation)
					writerResult <- err
				}
			}()
			waitForPostgresLockWait(t, ctx, pool, "pg_advisory_xact_lock")
			require.NoError(t, blocker.Commit(ctx))
			require.ErrorIs(t, <-freezeResult, spacelifecycle.ErrUnavailable)
			require.Error(t, <-writerResult)
		})
	}
}
