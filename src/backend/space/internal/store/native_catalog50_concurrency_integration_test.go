//go:build spacebanintegration

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Run on genuine qualified original45 and candidate50 source fixtures. The
// installer initializes this small real cohort before the two-session probe.
// Session B changes a covered function after A's first actual source callback,
// and restores it only after A's second callback has rejected the sampled drift.
// Both row statements are inside one outer DO/client command and native frame.
func TestNativeCatalogConcurrentRowDrift(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("SPACE_BAN_TEST_DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	st := &SpaceStore{Pool: pool}
	space := nativeBanSpace(19)
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
		return scoped.InitializeScopedNativeMembership(ctx, space)
	}))
	var beforeRows []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(m) ORDER BY profile_id) FROM space_members m WHERE space_id=$1`, space).Scan(&beforeRows))
	b, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer b.Release()
	_, err = b.Exec(ctx, `SELECT pg_advisory_lock(788100); SELECT pg_advisory_lock(788102)`)
	require.NoError(t, err)
	defer func() {
		cancel()
		cleanup, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		_, _ = b.Exec(cleanup, `ALTER FUNCTION native_membership_guard.terminal_cleanup() COST 100`)
		_, _ = b.Exec(cleanup, `SELECT pg_advisory_unlock_all()`)
	}()
	pid := make(chan int32, 1)
	result := make(chan error, 1)
	rollback := errors.New("native catalog concurrent probe rollback")
	go func() {
		result <- st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
			capture, err := scoped.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{nativeBanProfile(1), nativeBanProfile(3)})
			if err != nil {
				return err
			}
			if capture == nil || capture.dbMutation == nil {
				return errors.New("probe requires actual native full capture")
			}
			var backend int32
			if err = scoped.tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backend); err != nil {
				return err
			}
			pid <- backend
			code := fmt.Sprintf(`DO $probe$ DECLARE rejected boolean:=false; message text; BEGIN
 UPDATE public.space_members SET joined_at=joined_at WHERE space_id='%s' AND profile_id='%s';
 IF NOT FOUND THEN RAISE EXCEPTION 'first actual source row missing'; END IF;
 PERFORM pg_advisory_xact_lock(788101);
 PERFORM pg_advisory_xact_lock(788100);
 BEGIN
  UPDATE public.space_members SET joined_at=joined_at WHERE space_id='%s' AND profile_id='%s';
 EXCEPTION WHEN SQLSTATE '55000' THEN
  GET STACKED DIAGNOSTICS message=MESSAGE_TEXT;
  IF message<>'membership guard catalog is not exact' THEN RAISE; END IF;
  rejected:=true;
 END;
 IF NOT rejected THEN RAISE EXCEPTION 'second actual row callback missed concurrent catalog drift'; END IF;
 PERFORM pg_advisory_xact_lock(788103);
 PERFORM pg_advisory_xact_lock(788102);
 END $probe$`, space, nativeBanProfile(1), space, nativeBanProfile(3))
			if _, err = scoped.tx.Exec(ctx, code); err != nil {
				return err
			}
			if err = scoped.finishEffectiveMemberMutations(ctx, capture); err != nil {
				return err
			}
			return rollback
		})
	}()
	var backend int32
	select {
	case backend = <-pid:
	case err := <-result:
		t.Fatalf("actual native capture setup failed: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	waitBoundary := func(marker, barrier int64) {
		t.Helper()
		deadline := time.NewTimer(2 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			var ready bool
			err := b.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND objsubid=1 AND classid=0 AND objid=$2::oid AND granted)
 AND EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND objsubid=1 AND classid=0 AND objid=$3::oid AND NOT granted)`, backend, marker, barrier).Scan(&ready)
			require.NoError(t, err)
			if ready {
				return
			}
			select {
			case err := <-result:
				t.Fatalf("row callback failed before synchronized boundary: %v", err)
			case <-deadline.C:
				t.Fatal("actual row callback synchronization deadline")
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-ticker.C:
			}
		}
	}
	waitBoundary(788101, 788100)
	_, err = b.Exec(ctx, `ALTER FUNCTION native_membership_guard.terminal_cleanup() COST 101`)
	require.NoError(t, err, "covered drift commits after the first row callback")
	_, err = b.Exec(ctx, `SELECT pg_advisory_unlock(788100)`)
	require.NoError(t, err)
	waitBoundary(788103, 788102)
	_, err = b.Exec(ctx, `ALTER FUNCTION native_membership_guard.terminal_cleanup() COST 100`)
	require.NoError(t, err, "restore exact metadata after rejection but before native completion")
	_, err = b.Exec(ctx, `SELECT pg_advisory_unlock(788102)`)
	require.NoError(t, err)
	select {
	case err := <-result:
		require.ErrorIs(t, err, rollback, "exact catalog rejection was caught, restoration/completion succeeded, all row effects roll back")
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var frames int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames`).Scan(&frames))
	require.Zero(t, frames)
	var afterRows []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(m) ORDER BY profile_id) FROM space_members m WHERE space_id=$1`, space).Scan(&afterRows))
	require.Equal(t, beforeRows, afterRows)
}
