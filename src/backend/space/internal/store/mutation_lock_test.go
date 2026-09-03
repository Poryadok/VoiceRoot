package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func independentMutationLockPool(t *testing.T, source *pgxpool.Pool, maxConns int32) *pgxpool.Pool {
	t.Helper()
	config := source.Config()
	config.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	cleanupMutationLockPool(t, pool)
	require.NoError(t, pool.Ping(context.Background()))
	return pool
}

func cleanupMutationLockPool(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	t.Cleanup(func() {
		done := make(chan struct{})
		go func() {
			pool.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Log("timed out closing mutation lock pool; a failed implementation may still hold a leased connection")
		}
	})
}

type mutationLockResult struct {
	release func()
	err     error
}

func cleanupMutationLock(t *testing.T, release func()) func() {
	t.Helper()
	var once sync.Once
	wrapped := func() { once.Do(release) }
	t.Cleanup(wrapped)
	return wrapped
}

func acquireMutationLockAsync(ctx context.Context, locker *SpaceMutationLocker, spaceID uuid.UUID) <-chan mutationLockResult {
	result := make(chan mutationLockResult, 1)
	go func() {
		release, err := locker.Acquire(ctx, spaceID)
		result <- mutationLockResult{release: release, err: err}
	}()
	return result
}

func requireMutationLockResult(t *testing.T, result <-chan mutationLockResult) mutationLockResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(3 * time.Second):
		t.Fatal("space mutation lock did not return")
		return mutationLockResult{}
	}
}

func TestSpaceMutationLocker_SerializesSameSpaceAcrossPoolsButNotDifferentSpaces(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	mainPool := startSpacePostgresForStoreTest(t, ctx)
	lockPoolA := independentMutationLockPool(t, mainPool, 2)
	lockPoolB := independentMutationLockPool(t, mainPool, 2)
	lockerA := NewSpaceMutationLocker(lockPoolA)
	lockerB := NewSpaceMutationLocker(lockPoolB)
	spaceA := uuid.New()
	spaceB := uuid.New()

	releaseA, err := lockerA.Acquire(ctx, spaceA)
	require.NoError(t, err)
	require.NotNil(t, releaseA)
	releaseA = cleanupMutationLock(t, releaseA)

	waitCtx, cancelWait := context.WithTimeout(ctx, 2*time.Second)
	defer cancelWait()
	waitingForA := acquireMutationLockAsync(waitCtx, lockerB, spaceA)
	select {
	case got := <-waitingForA:
		if got.release != nil {
			got.release()
		}
		t.Fatalf("same-space lock from another pool acquired before release: %v", got.err)
	case <-time.After(150 * time.Millisecond):
	}

	differentCtx, cancelDifferent := context.WithTimeout(ctx, time.Second)
	defer cancelDifferent()
	releaseB, err := lockerB.Acquire(differentCtx, spaceB)
	require.NoError(t, err, "a different space must not wait behind space A")
	releaseB()

	releaseA()
	got := requireMutationLockResult(t, waitingForA)
	require.NoError(t, got.err)
	require.NotNil(t, got.release)
	got.release()
}

func TestSpaceMutationLocker_CanceledWaiterDoesNotLeakOrPoisonConnection(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	mainPool := startSpacePostgresForStoreTest(t, ctx)
	lockPoolA := independentMutationLockPool(t, mainPool, 1)
	lockPoolB := independentMutationLockPool(t, mainPool, 1)
	lockPoolC := independentMutationLockPool(t, mainPool, 1)
	lockerA := NewSpaceMutationLocker(lockPoolA)
	lockerB := NewSpaceMutationLocker(lockPoolB)
	lockerC := NewSpaceMutationLocker(lockPoolC)
	spaceID := uuid.New()
	var waiterBackendPID int
	require.NoError(t, lockPoolB.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&waiterBackendPID))

	releaseA, err := lockerA.Acquire(ctx, spaceID)
	require.NoError(t, err)
	require.NotNil(t, releaseA)
	releaseA = cleanupMutationLock(t, releaseA)

	waitCtx, cancelWait := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancelWait()
	_, err = lockerB.Acquire(waitCtx, spaceID)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	queryCtx, cancelQuery := context.WithTimeout(ctx, time.Second)
	defer cancelQuery()
	var replacementBackendPID int
	require.NoError(t, lockPoolB.QueryRow(queryCtx, "SELECT pg_backend_pid()").Scan(&replacementBackendPID), "canceled acquisition must reconnect through a healthy session")
	require.NotEqual(t, waiterBackendPID, replacementBackendPID, "ambiguous advisory-lock acquisition must destroy its PostgreSQL session")

	releaseA()
	reacquireCtx, cancelReacquire := context.WithTimeout(ctx, time.Second)
	defer cancelReacquire()
	releaseB, err := lockerB.Acquire(reacquireCtx, spaceID)
	require.NoError(t, err, "the canceled waiter must be able to reacquire after the holder releases")
	releaseB()

	// A session-level advisory lock is re-entrant. Acquiring again through B alone
	// could therefore hide a leaked acquisition count. A third pool proves that
	// B's canceled/retried session left no advisory lock behind.
	probeCtx, cancelProbe := context.WithTimeout(ctx, time.Second)
	defer cancelProbe()
	releaseC, err := lockerC.Acquire(probeCtx, spaceID)
	require.NoError(t, err, "no orphaned advisory lock may remain on the canceled waiter's session")
	releaseC()
}

func TestSpaceMutationLocker_CanceledHolderContextReleaseAllowsThirdSessionReacquire(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	mainPool := startSpacePostgresForStoreTest(t, ctx)
	lockPoolA := independentMutationLockPool(t, mainPool, 1)
	lockPoolB := independentMutationLockPool(t, mainPool, 1)
	lockPoolC := independentMutationLockPool(t, mainPool, 1)
	lockerA := NewSpaceMutationLocker(lockPoolA)
	lockerB := NewSpaceMutationLocker(lockPoolB)
	lockerC := NewSpaceMutationLocker(lockPoolC)
	spaceID := uuid.New()

	holderCtx, cancelHolder := context.WithCancel(ctx)
	releaseA, err := lockerA.Acquire(holderCtx, spaceID)
	require.NoError(t, err)
	require.NotNil(t, releaseA)
	releaseA = cleanupMutationLock(t, releaseA)
	cancelHolder()
	require.ErrorIs(t, holderCtx.Err(), context.Canceled)

	blockedCtx, cancelBlocked := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancelBlocked()
	_, err = lockerB.Acquire(blockedCtx, spaceID)
	require.ErrorIs(t, err, context.DeadlineExceeded, "canceling the holder context must not release its session lock early")

	releaseA()
	probeCtx, cancelProbe := context.WithTimeout(ctx, time.Second)
	defer cancelProbe()
	releaseC, err := lockerC.Acquire(probeCtx, spaceID)
	require.NoError(t, err, "release must use detached cleanup after the holder context is canceled")
	releaseC()
}
