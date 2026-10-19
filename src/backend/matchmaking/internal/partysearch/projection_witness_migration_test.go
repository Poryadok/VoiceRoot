package partysearch

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
	"voice/backend/matchmaking/internal/mmadmission"
)

func projectionMigration(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/matchmaking_db/000026_mm_native_response." + direction + ".sql")
	require.NoError(t, err)
	return string(raw)
}

func TestProjectionWitnessDownRefusesPinnedRRBeforeHeldGate(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	rr, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer rr.Rollback(ctx)
	var count int
	require.NoError(t, rr.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_head`).Scan(&count))
	require.Zero(t, count)
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_admission_head(protocol_version,state)VALUES(1,'CLOSED')`)
	require.NoError(t, err)
	closer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer closer.Rollback(ctx)
	require.NoError(t, mmadmission.AcquireCloseGate(ctx, closer))
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err = rr.Exec(bounded, projectionMigration(t, "down"))
	require.ErrorContains(t, err, "requires fresh read committed checks", "RR must be refused before waiting for the held gate or consulting its hidden row")
	require.NoError(t, rr.Rollback(ctx))
	require.NoError(t, closer.Rollback(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_head`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestProjectionWitnessDownSerializesNoncooperatingFirstWriter(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(ctx)
	_, err = writer.Exec(ctx, `INSERT INTO mm_native_admission_head(protocol_version,state)VALUES(1,'CLOSED')`)
	require.NoError(t, err)
	reader, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer reader.Release()
	var pid int
	require.NoError(t, reader.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	sql := projectionMigration(t, "down")
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		_, e := reader.Exec(bounded, sql)
		done <- e
	}()
	require.Eventually(t, func() bool {
		select {
		case e := <-done:
			t.Fatalf("DOWN bypassed pending first writer: %v", e)
		default:
		}
		var waiting bool
		e := pool.QueryRow(ctx, `SELECT wait_event_type='Lock' FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&waiting)
		return e == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.ErrorContains(t, <-done, "evidence must be retained")
	_, err = reader.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
	var retained bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='matchmaking_party_search_queue_intents' AND column_name='projected_state_sha256')`).Scan(&retained))
	require.True(t, retained)
}

func TestProjectionWitnessEmptyRCRoundTripAndUnknownWitnessRetention(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, projectionMigration(t, "down"))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, projectionMigration(t, "up"))
	require.NoError(t, err)
	p := partyParams(t, pool)
	_, err = pool.Exec(ctx, `INSERT INTO parties(id,leader_profile_id,game_id,mode,criteria)VALUES(gen_random_uuid(),$1,$2,$3,$4::jsonb)`, p.InitiatorProfileID, p.GameID, p.Mode, p.Criteria)
	require.NoError(t, err)
	// An unknown retained witness cannot be erased merely because no admitted
	// head exists. This row is a retention oracle, never installation authority.
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_party_search_queue_intents(party_id,revision,action,projected_at,projected_state_sha256)SELECT id,1,'RELEASE',clock_timestamp(),decode(repeat('01',32),'hex') FROM parties LIMIT 1`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, projectionMigration(t, "down"))
	require.ErrorContains(t, err, "evidence must be retained")
	_, err = conn.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
}

func TestProjectionWitnessUpgradeRequiresClosedAdmission(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, projectionMigration(t, "down"))
	require.NoError(t, err)
	insertAdmissionFixture(t, pool, admissionFixture())
	_, err = conn.Exec(ctx, projectionMigration(t, "up"))
	require.ErrorContains(t, err, "close MM native admission before schema transition")
	_, err = conn.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, projectionMigration(t, "up"))
	require.NoError(t, err)
}
