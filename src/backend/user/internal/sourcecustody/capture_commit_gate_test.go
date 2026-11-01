package sourcecustody

import (
	"context"
	"crypto/sha256"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/testsupport/associationfixture"
)

type commitPIDTracer struct {
	*capturePreReadTrace
	armed     atomic.Bool
	commitPID chan uint32
}

func (t *commitPIDTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.EqualFold(strings.TrimSpace(data.SQL), "commit") && t.armed.Load() {
		select {
		case t.commitPID <- conn.PgConn().PID():
		default:
		}
	}
	return t.capturePreReadTrace.TraceQueryStart(ctx, conn, data)
}

// The trigger is test-only and gates an actual User COMMIT after COPY inserted
// a retained association. Synthetic OPEN is not physical custody evidence.
func TestCaptureClientHoldsGrantWhileUserCommitWaits(t *testing.T) {
	ctx := context.Background()
	root := custodyDatabase(t, ctx)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", root.Config().ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	trace := &capturePreReadTrace{started: make(chan uint32, 4), commitStarted: make(chan struct{}), commitRelease: make(chan struct{})}
	commitTrace := &commitPIDTracer{capturePreReadTrace: trace, commitPID: make(chan uint32, 1)}
	cfg := db.Pool.Config().Copy()
	cfg.ConnConfig.Tracer = commitTrace
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	fingerprint := sha256.Sum256(receipt.UserKey)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: fingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	store, err := identityassociation.NewStore(pool, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(ctx))
	require.NoError(t, store.ActivateGuards(ctx))
	var history, source uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&history, &source))
	binding := custodyBinding()
	binding.HistoryGeneration, binding.UserSourceGeneration = history, source
	binding.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	f := newStreamFixtureBound(t, false, time.Now, 100*time.Millisecond, &binding, nil)
	installStreamPhysical(t, f, db.Pool)
	client, err := NewClient(f.client, f.issuer, 500*time.Millisecond)
	require.NoError(t, err)
	capture, err := NewCaptureClient(client, binding, store)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `CREATE FUNCTION public.capture_test_commit_gate() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(819714312471::bigint); RETURN NEW; END $$;
CREATE CONSTRAINT TRIGGER capture_test_commit_gate AFTER INSERT ON public.identity_associations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.capture_test_commit_gate()`)
	require.NoError(t, err)
	blocker, err := db.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer blocker.Release()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_lock(819714312471::bigint)`)
	require.NoError(t, err)
	var unlockOnce sync.Once
	unlock := func() {
		unlockOnce.Do(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var ok bool
			require.NoError(t, blocker.QueryRow(cleanup, `SELECT pg_advisory_unlock(819714312471::bigint)`).Scan(&ok))
			require.True(t, ok)
		})
	}
	defer unlock()
	type outcome struct {
		result identityassociation.CaptureBatchResult
		err    error
	}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	results := make(chan outcome, 1)
	joined := false
	commitTrace.armed.Store(true)
	go func() { r, e := capture.CaptureProfiles(call, db.Profiles); results <- outcome{r, e} }()
	defer func() {
		unlock()
		cancel()
		if !joined {
			select {
			case <-results:
			case <-time.After(5 * time.Second):
				t.Error("capture failed to join")
			}
		}
	}()
	var pid uint32
	select {
	case <-trace.started:
	case got := <-results:
		joined = true
		t.Fatalf("capture returned before pre-read: %v", got.err)
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("pre-read not observed")
	}
	select {
	case pid = <-commitTrace.commitPID:
	case got := <-results:
		joined = true
		t.Fatalf("capture returned before COMMIT: %v", got.err)
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("COMMIT not observed")
	}
	var waiting bool
	require.Eventually(t, func() bool {
		return db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND state='active' AND lower(query)='commit' AND wait_event_type='Lock' AND wait_event='advisory')`, pid).Scan(&waiting) == nil && waiting
	}, 1500*time.Millisecond, 10*time.Millisecond, "observe actual PostgreSQL COMMIT waiting on fixture gate")
	var shared int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid`).Scan(&shared))
	require.Equal(t, 1, shared, "permit must remain held while PostgreSQL COMMIT is blocked")
	select {
	case got := <-results:
		joined = true
		t.Fatalf("capture returned while COMMIT blocked: %v", got.err)
	default:
	}
	unlock()
	select {
	case got := <-results:
		joined = true
		require.NoError(t, got.err)
		require.Equal(t, identityassociation.CaptureAssociated, got.result.Observations[db.Profiles[0]].Status)
	case <-time.After(2 * time.Second):
		t.Fatal("capture did not finish after gate released")
	}
	require.Eventually(t, func() bool {
		return f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid`).Scan(&shared) == nil && shared == 0
	}, 2*time.Second, 10*time.Millisecond, "permit released after capture returns")
}
