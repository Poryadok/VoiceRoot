package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/spacelifecycle"
)

func p3DownSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/voice_db/000014_space_media_p3.down.sql")
	require.NoError(t, err)
	return string(raw)
}
func execP3Down(ctx context.Context, pool *pgxpool.Pool, sql string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, err = conn.Exec(ctx, sql)
	if err != nil {
		_, _ = conn.Exec(ctx, "ROLLBACK")
	}
	return err
}

func TestP3DownRefusesRetainedGrantTargetsAndReceipt(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	admissions := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	a.Identity, _ = livekit.SpaceParticipantIdentity(a.ProfileID.String(), a.Generation)
	require.NoError(t, prepareAdmission(ctx, admissions, &a))
	require.NoError(t, admissions.Fence(ctx, a))
	require.NoError(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	grant := livekit.SpaceRoomGrant{AccountID: a.AccountID.String(), ProfileID: a.ProfileID.String(), SpaceID: a.SpaceID.String(), VoiceRoomID: a.VoiceRoomID, LiveKitRoomName: "voice-room-" + a.VoiceRoomID, MediaGeneration: a.Generation, SessionEpoch: a.SessionEpoch, AccessEpoch: a.AccessEpoch, PolicyEpoch: a.PolicyEpoch, CanJoin: true, CanSubscribe: true}
	_, _, err := admissions.ReserveSpaceGrant(ctx, uuid.New(), a.OperationID, a.Generation, grant)
	require.NoError(t, err)
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: a.SpaceID.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}
	_, err = spacelifecycle.NewPostgresStore(pool, BeginPartyRoomClose).ApplyFence(ctx, fence, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error {
		return spacelifecycle.ErrUnavailable
	})
	require.Error(t, err)
	snapshot := func() string {
		var value string
		require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_build_object('grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY grant_id) FROM voice_space_media_grants g),'targets',(SELECT jsonb_agg(to_jsonb(t) ORDER BY target_id) FROM voice_space_lifecycle_targets t),'heads',(SELECT jsonb_agg(to_jsonb(h) ORDER BY voice_room_id) FROM voice_space_media_room_heads h),'admissions',(SELECT jsonb_agg(to_jsonb(a) ORDER BY operation_id) FROM voice_space_media_admissions a),'receipts',(SELECT jsonb_agg(to_jsonb(r) ORDER BY space_id,generation) FROM voice_space_lifecycle_fence_receipts r))::text`).Scan(&value))
		return value
	}
	before := snapshot()
	require.Error(t, execP3Down(ctx, pool, p3DownSQL(t)), "rollback must refuse retained credential/target evidence")
	require.Equal(t, before, snapshot(), "refusal must preserve every retained row/column")
}

func TestP3DownEmptyRoundtrip(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	require.NoError(t, execP3Down(ctx, pool, p3DownSQL(t)))
	up, err := os.ReadFile("../../../migrations/voice_db/000014_space_media_p3.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_name='voice_space_lifecycle_fence_receipts' AND column_name IN ('coverage_sealed','retirement_proven')`).Scan(&count))
	require.Equal(t, 2, count)
}

func TestP3DownSerializesWithUncommittedNativeWriter(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(ctx)
	_, err = writer.Exec(ctx, `INSERT INTO voice_space_media_room_heads(voice_room_id,room_generation,space_id,call_room_id,creator_operation_id,state) VALUES($1,1,$2,$3,$4,'OPEN')`, uuid.New(), uuid.New(), uuid.NewString(), uuid.New())
	require.NoError(t, err)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	pid := conn.Conn().PgConn().PID()
	done := make(chan error, 1)
	sql := p3DownSQL(t)
	go func() {
		_, err := conn.Exec(ctx, sql)
		if err != nil {
			_, _ = conn.Exec(ctx, "ROLLBACK")
		}
		done <- err
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND NOT granted)`, pid).Scan(&blocked)
		return err == nil && blocked
	}, 10*time.Second, 10*time.Millisecond, "actual DOWN must wait behind the uncommitted native writer")
	require.NoError(t, writer.Commit(ctx))
	select {
	case err := <-done:
		require.Error(t, err, "committed evidence must be seen before destructive DDL")
	case <-time.After(10 * time.Second):
		t.Fatal("DOWN did not finish after writer committed")
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_room_heads WHERE lifecycle_generation=0`).Scan(&count))
	require.Equal(t, 1, count)
	var grants string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('voice_space_media_grants')::text`).Scan(&grants))
	require.NotEmpty(t, grants)
}
