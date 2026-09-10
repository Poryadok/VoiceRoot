package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// This cycle deliberately needs no Role migrations or v2 ledger schema: it
// establishes the transaction boundary shared by ordinary and transition work.
func TestWithLockedTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := StartRoleDBForStoreTest(t, ctx)
	defer pool.Close()
	s := &RoleStore{Pool: pool}
	_, err := pool.Exec(ctx, "CREATE TABLE scope_probe (id UUID PRIMARY KEY)")
	require.NoError(t, err)
	spaceA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	spaceB := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	t.Run("commit rollback and covered nested executor", func(t *testing.T) {
		marker := uuid.New()
		op := uuid.New()
		var captured pgx.Tx
		err := s.withLockedTransaction(ctx, op, []uuid.UUID{spaceB, spaceA, spaceA}, func(scoped *RoleStore) error {
			require.NotNil(t, scoped.tx)
			captured = scoped.tx
			_, err := scoped.db().Exec(ctx, "INSERT INTO scope_probe VALUES ($1)", marker)
			if err != nil {
				return err
			}
			var outerXID int64
			if err = scoped.db().QueryRow(ctx, "SELECT txid_current()").Scan(&outerXID); err != nil {
				return err
			}
			// Pool queries cannot see an uncommitted write from the callback.
			var outside int
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM scope_probe WHERE id=$1", marker).Scan(&outside))
			require.Zero(t, outside)
			for _, nestedOp := range []uuid.UUID{uuid.Nil, op} {
				err = scoped.withLockedTransaction(ctx, nestedOp, []uuid.UUID{spaceA}, func(nested *RoleStore) error {
					require.Equal(t, scoped.tx, nested.tx)
					var innerXID int64
					if e := nested.db().QueryRow(ctx, "SELECT txid_current()").Scan(&innerXID); e != nil {
						return e
					}
					require.Equal(t, outerXID, innerXID)
					rows, e := nested.db().Query(ctx, "SELECT id FROM scope_probe WHERE id=$1", marker)
					if e != nil {
						return e
					}
					defer rows.Close()
					require.True(t, rows.Next())
					var got uuid.UUID
					require.NoError(t, rows.Scan(&got))
					require.Equal(t, marker, got)
					require.False(t, rows.Next())
					return rows.Err()
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)
		require.ErrorIs(t, captured.Commit(ctx), pgx.ErrTxClosed, "helper owns commit")
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM scope_probe WHERE id=$1", marker).Scan(&count))
		require.Equal(t, 1, count)

		rolledBack := uuid.New()
		sentinel := errors.New("callback rejected operation")
		err = s.withLockedTransaction(ctx, uuid.Nil, []uuid.UUID{spaceA}, func(scoped *RoleStore) error {
			captured = scoped.tx
			_, e := scoped.db().Exec(ctx, "INSERT INTO scope_probe VALUES ($1)", rolledBack)
			if e != nil {
				return e
			}
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)
		require.ErrorIs(t, captured.Commit(ctx), pgx.ErrTxClosed, "helper owns rollback")
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM scope_probe WHERE id=$1", rolledBack).Scan(&count))
		require.Zero(t, count)
	})

	t.Run("nested scope cannot widen or introduce operation lock", func(t *testing.T) {
		for _, outerOp := range []uuid.UUID{uuid.Nil, uuid.New()} {
			err := s.withLockedTransaction(ctx, outerOp, []uuid.UUID{spaceA}, func(scoped *RoleStore) error {
				for _, tc := range []struct {
					op     uuid.UUID
					spaces []uuid.UUID
				}{
					{outerOp, []uuid.UUID{spaceA, spaceB}},
					{uuid.New(), []uuid.UUID{spaceA}},
				} {
					called := false
					nestedErr := scoped.withLockedTransaction(ctx, tc.op, tc.spaces, func(*RoleStore) error { called = true; return nil })
					require.Error(t, nestedErr)
					require.False(t, called, "invalid scope must fail before callback")
				}
				return nil
			})
			require.NoError(t, err)
		}
	})

	t.Run("operation lock precedes every space lock", func(t *testing.T) {
		op := uuid.New()
		blocker, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = blocker.Rollback(context.Background()) }()
		_, err = blocker.Exec(ctx, "SELECT pg_advisory_xact_lock($1::integer, hashtext($2))", int32(0x524f5032), op.String())
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() {
			done <- s.withLockedTransaction(ctx, op, []uuid.UUID{spaceA}, func(*RoleStore) error { return nil })
		}()
		waitForScopeLockWaiter(t, ctx, pool)
		assertScopeSpaceAvailable(t, ctx, pool, spaceA)
		require.NoError(t, blocker.Commit(ctx))
		require.NoError(t, awaitScopeResult(t, ctx, done))
	})

	t.Run("reverse input order still waits on smallest space first", func(t *testing.T) {
		blocker, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = blocker.Rollback(context.Background()) }()
		_, err = blocker.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", spaceA.String())
		require.NoError(t, err)
		done := make(chan error, 2)
		for _, spaces := range [][]uuid.UUID{{spaceB, spaceA, spaceB}, {spaceA, spaceB}} {
			go func(ids []uuid.UUID) {
				done <- s.withLockedTransaction(ctx, uuid.Nil, ids, func(*RoleStore) error { return nil })
			}(spaces)
		}
		require.Eventually(t, func() bool {
			var count int
			e := pool.QueryRow(ctx, "SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted").Scan(&count)
			return e == nil && count == 2
		}, 5*time.Second, 10*time.Millisecond)
		assertScopeSpaceAvailable(t, ctx, pool, spaceB)
		require.NoError(t, blocker.Commit(ctx))
		require.NoError(t, awaitScopeResult(t, ctx, done))
		require.NoError(t, awaitScopeResult(t, ctx, done))
	})

	t.Run("blocked space cancellation never enters callback or retains lock", func(t *testing.T) {
		blocker, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = blocker.Rollback(context.Background()) }()
		_, err = blocker.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", spaceA.String())
		require.NoError(t, err)
		blockedCtx, stop := context.WithCancel(ctx)
		defer stop()
		called := make(chan struct{}, 1)
		done := make(chan error, 1)
		go func() {
			done <- s.withLockedTransaction(blockedCtx, uuid.Nil, []uuid.UUID{spaceA}, func(*RoleStore) error { called <- struct{}{}; return nil })
		}()
		waitForScopeLockWaiter(t, ctx, pool)
		stop()
		require.ErrorIs(t, awaitScopeResult(t, ctx, done), context.Canceled)
		require.Empty(t, called)
		require.NoError(t, blocker.Commit(ctx))
		require.NoError(t, s.withLockedTransaction(ctx, uuid.Nil, []uuid.UUID{spaceA}, func(*RoleStore) error { return nil }))
	})

	t.Run("callback cancellation rolls back its write", func(t *testing.T) {
		callbackCtx, stop := context.WithCancel(ctx)
		defer stop()
		marker := uuid.New()
		err := s.withLockedTransaction(callbackCtx, uuid.Nil, []uuid.UUID{spaceA}, func(scoped *RoleStore) error {
			_, e := scoped.db().Exec(callbackCtx, "INSERT INTO scope_probe VALUES ($1)", marker)
			if e != nil {
				return e
			}
			stop()
			return nil
		})
		require.Error(t, err, "canceled callback context cannot commit")
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM scope_probe WHERE id=$1", marker).Scan(&count))
		require.Zero(t, count)
		assertScopeSpaceAvailable(t, ctx, pool, spaceA)
	})
}

func waitForScopeLockWaiter(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted)").Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "transaction must reach a real PostgreSQL lock wait")
}

func assertScopeSpaceAvailable(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID uuid.UUID) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	var available bool
	require.NoError(t, tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtext($1))", spaceID.String()).Scan(&available))
	require.True(t, available)
}

func awaitScopeResult(t *testing.T, ctx context.Context, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Fatal("scope worker did not finish: ", ctx.Err())
		return ctx.Err()
	}
}
