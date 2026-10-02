package store

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/pkg/spacemutationlock"
)

// SpaceMutationLocker holds a PostgreSQL session advisory lock for the full
// duration of a space mutation. Its pool must be dedicated to leases: every
// held lock pins one connection until the returned release function runs. The
// connection must be direct PostgreSQL or session-pooled; transaction-mode
// poolers cannot preserve session advisory-lock ownership.
type SpaceMutationLocker struct {
	pool *pgxpool.Pool
}

func NewSpaceMutationLocker(pool *pgxpool.Pool) *SpaceMutationLocker {
	return &SpaceMutationLocker{pool: pool}
}

// Acquire obtains a session-level advisory lock scoped to spaceID. Callers
// must invoke the returned function; it is safe to invoke more than once.
func (l *SpaceMutationLocker) Acquire(ctx context.Context, spaceID uuid.UUID) (func(), error) {
	if l == nil || l.pool == nil {
		return nil, errors.New("space mutation lock pool not configured")
	}

	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	key := spaceMutationAdvisoryKey(spaceID)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		// Cancellation can race with a successful server-side acquisition. The
		// result is ambiguous, so destroy the PostgreSQL session instead of
		// returning it to the pool with a possibly held advisory lock.
		closeHijackedMutationLockConn(ctx, conn)
		return nil, err
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			cleanupCtx, cancel := BoundedCleanupContext(ctx)
			defer cancel()

			var unlocked bool
			err := conn.QueryRow(cleanupCtx, "SELECT pg_advisory_unlock($1)", key).Scan(&unlocked)
			if err == nil && unlocked {
				conn.Release()
				return
			}
			// A failed or negative unlock leaves the connection's lock state
			// uncertain. Closing the session is the only safe release.
			closeHijackedMutationLockConn(cleanupCtx, conn)
		})
	}, nil
}

func spaceMutationAdvisoryKey(spaceID uuid.UUID) int64 {
	return spacemutationlock.Key(spaceID)
}

func closeHijackedMutationLockConn(ctx context.Context, conn *pgxpool.Conn) {
	cleanupCtx, cancel := BoundedCleanupContext(ctx)
	defer cancel()
	_ = conn.Hijack().Close(cleanupCtx)
}
