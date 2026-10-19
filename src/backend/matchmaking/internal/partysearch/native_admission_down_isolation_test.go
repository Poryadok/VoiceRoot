package partysearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	admission "voice/backend/matchmaking/internal/mmadmission"
)

func requireMM024IsolationRefusal(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr, "DOWN must refuse isolation, not erase a hidden retained marker")
	require.Equal(t, "P0001", pgErr.Code)
	require.Equal(t, "MM native rollback requires fresh read committed checks", pgErr.Message)
}

func TestNativeAdmissionDownIsolationRefusesPinnedRR(t *testing.T) {
	pool := partyDBThrough(t, "000024")
	ctx := context.Background()
	rr, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer rr.Rollback(ctx)
	var count int
	require.NoError(t, rr.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_head`).Scan(&count))
	require.Zero(t, count, "pin an actually empty RR snapshot")
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_admission_head(protocol_version,state) VALUES(1,'CLOSED')`)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_head`).Scan(&count))
	require.Equal(t, 1, count, "independent writer actually committed the retained marker")
	require.NoError(t, rr.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_head`).Scan(&count))
	require.Zero(t, count, "retained marker is genuinely hidden in the pinned RR snapshot")
	t.Log("pinned RR sees 0; independent committed view sees retained CLOSED=1")
	down, err := os.ReadFile("../../../migrations/matchmaking_db/000024_mm_native_admission.down.sql")
	require.NoError(t, err)
	_, err = rr.Exec(ctx, string(down))
	if err == nil {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.mm_native_admission_head') IS NOT NULL`).Scan(&exists))
		t.Logf("DOWN returned nil after pinned RR; retained marker relation still exists=%v", exists)
	}
	requireMM024IsolationRefusal(t, err)
	require.NoError(t, rr.Rollback(ctx))
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM mm_native_admission_head`).Scan(&state))
	require.Equal(t, "CLOSED", state)
}

func TestNativeAdmissionDownIsolationRefusesRRBeforeCloseGate(t *testing.T) {
	pool := partyDBThrough(t, "000024")
	ctx := context.Background()
	rr, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer rr.Rollback(ctx)
	var count int
	require.NoError(t, rr.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_head`).Scan(&count))
	closer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer closer.Rollback(ctx)
	require.NoError(t, admission.AcquireCloseGate(ctx, closer))
	down, err := os.ReadFile("../../../migrations/matchmaking_db/000024_mm_native_admission.down.sql")
	require.NoError(t, err)
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err = rr.Exec(bounded, string(down))
	requireMM024IsolationRefusal(t, err)
	require.NoError(t, rr.Rollback(ctx))
	require.NoError(t, closer.Rollback(ctx))
}

func TestNativeAdmissionDownIsolationReadCommittedRoundTrip(t *testing.T) {
	pool := partyDBThrough(t, "000024")
	ctx := context.Background()
	down, err := os.ReadFile("../../../migrations/matchmaking_db/000024_mm_native_admission.down.sql")
	require.NoError(t, err)
	up, err := os.ReadFile("../../../migrations/matchmaking_db/000024_mm_native_admission.up.sql")
	require.NoError(t, err)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, `BEGIN ISOLATION LEVEL READ COMMITTED`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(down))
	require.NoError(t, err, "empty genuine RC rollback remains supported")
	var exists bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT to_regclass('public.mm_native_admission_head') IS NOT NULL`).Scan(&exists))
	require.False(t, exists)
	_, err = conn.Exec(ctx, string(up))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO mm_native_admission_head(protocol_version,state) VALUES(1,'CLOSED')`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(down))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "MM native admission marker must be retained", pgErr.Message)
	_, err = conn.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
	var state string
	require.NoError(t, conn.QueryRow(ctx, `SELECT state FROM mm_native_admission_head`).Scan(&state))
	require.Equal(t, "CLOSED", state)
}
