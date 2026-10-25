package spacemedia

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSpaceAdmissionOwnershipDownRetainsConcurrentFirstOwner(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	down, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000015_space_admission_ownership.down.sql"))
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Rollback(ctx) })
	operation := uuid.New()
	var expected string
	err = writer.QueryRow(ctx, `INSERT INTO voice_space_media_admission_owners
 (operation_id,space_id,voice_room_id,owner_token,owner_epoch,lease_until)
 VALUES($1,$2,$3,$4,1,clock_timestamp()+interval '1 minute')
 RETURNING to_jsonb(voice_space_media_admission_owners)::text`, operation, uuid.New(), uuid.New(), uuid.New()).Scan(&expected)
	require.NoError(t, err)
	rollback, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer rollback.Release()
	var pid int
	require.NoError(t, rollback.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	go func() {
		_, downErr := rollback.Exec(ctx, string(down))
		done <- downErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks
 WHERE pid=$1 AND relation='voice_space_media_admission_owners'::regclass
 AND mode='AccessExclusiveLock' AND NOT granted)`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "actual DOWN must wait behind the uncommitted first owner")
	require.NoError(t, writer.Commit(ctx))
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("DOWN did not finish after the first owner committed")
	}
	// Explicit transactional DOWN may leave this dedicated connection aborted.
	// Release its locks before independently observing the retained table/row.
	_, _ = rollback.Exec(ctx, "ROLLBACK")
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('voice_space_media_admission_owners') IS NOT NULL`).Scan(&exists))
	t.Logf("DOWN error=%v; ownership table retained=%v", err, exists)
	require.ErrorContains(t, err, "cannot remove admission ownership")
	require.True(t, exists)
	var actual string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM voice_space_media_admission_owners o WHERE operation_id=$1`, operation).Scan(&actual))
	require.Equal(t, expected, actual, "every committed ownership field survives refused DOWN")
}

func TestSpaceAdmissionOwnershipEmptyDownUp(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	root := filepath.Join("..", "..", "..", "migrations", "voice_db")
	for _, suffix := range []string{"down", "up"} {
		body, err := os.ReadFile(filepath.Join(root, "000015_space_admission_ownership."+suffix+".sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admission_owners`).Scan(&count))
	require.Zero(t, count)
}
