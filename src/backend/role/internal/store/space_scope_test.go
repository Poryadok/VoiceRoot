package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// The minimal fixture models only the agreed fence columns; ledger state
// transitions and the production migration belong to the ledger tests.
func TestWithinSpaces(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := StartRoleDBForStoreTest(t, ctx)
	defer pool.Close()
	_, err := pool.Exec(ctx, `
 CREATE TABLE ownership_transfer_v2 (space_id UUID NOT NULL, state TEXT NOT NULL);
 CREATE TABLE role_space_lifecycle (space_id UUID PRIMARY KEY, retired_at TIMESTAMPTZ);
 CREATE TABLE ordinary_scope_probe (id UUID PRIMARY KEY);`)
	require.NoError(t, err)
	s := &RoleStore{Pool: pool}

	t.Run("absent fence permits commit and nested subset retains executor", func(t *testing.T) {
		space, marker := uuid.New(), uuid.New()
		var retained scopeExecutor
		err := s.WithinSpaces(ctx, []uuid.UUID{space, uuid.New()}, func(scoped *RoleStore) error {
			retained = scoped.db()
			_, e := scoped.db().Exec(ctx, "INSERT INTO ordinary_scope_probe VALUES ($1)", marker)
			if e != nil {
				return e
			}
			return scoped.WithinSpaces(ctx, []uuid.UUID{space}, func(nested *RoleStore) error {
				require.Equal(t, scoped.tx, nested.tx)
				var found bool
				e := nested.db().QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM ordinary_scope_probe WHERE id=$1)", marker).Scan(&found)
				require.True(t, found)
				return e
			})
		})
		require.NoError(t, err)
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM ordinary_scope_probe WHERE id=$1", marker).Scan(&count))
		require.Equal(t, 1, count)
		_, err = retained.Exec(ctx, "INSERT INTO ordinary_scope_probe VALUES ($1)", uuid.New())
		require.ErrorIs(t, err, pgx.ErrTxClosed, "escaped executor must not use Pool")
		rows, err := retained.Query(ctx, "SELECT 1")
		if rows != nil {
			rows.Close()
		}
		require.ErrorIs(t, err, pgx.ErrTxClosed)
		require.ErrorIs(t, retained.QueryRow(ctx, "SELECT 1").Scan(&count), pgx.ErrTxClosed)
	})

	t.Run("prepared or retired space denies whole batch before callback", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			retired bool
			want    error
		}{
			{"prepared", false, ErrSpaceFrozen}, {"retired", true, ErrSpaceRetired},
		} {
			t.Run(tc.name, func(t *testing.T) {
				blocked, allowed, marker := uuid.New(), uuid.New(), uuid.New()
				if tc.retired {
					_, err = pool.Exec(ctx, "INSERT INTO role_space_lifecycle VALUES ($1, now())", blocked)
				} else {
					_, err = pool.Exec(ctx, "INSERT INTO ownership_transfer_v2 VALUES ($1, 'prepared')", blocked)
				}
				require.NoError(t, err)
				for _, spaces := range [][]uuid.UUID{{allowed, blocked}, {blocked, allowed}} {
					called := false
					err = s.WithinSpaces(ctx, spaces, func(scoped *RoleStore) error {
						called = true
						_, e := scoped.db().Exec(ctx, "INSERT INTO ordinary_scope_probe VALUES ($1)", marker)
						return e
					})
					require.ErrorIs(t, err, tc.want)
					require.False(t, called)
				}
				var count int
				require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM ordinary_scope_probe WHERE id=$1", marker).Scan(&count))
				require.Zero(t, count)
			})
		}
	})

	t.Run("terminal receipts and live lifecycle rows allow ordinary work", func(t *testing.T) {
		for _, state := range []string{"finalized", "aborted"} {
			space := uuid.New()
			_, err := pool.Exec(ctx, "INSERT INTO ownership_transfer_v2 VALUES ($1, $2)", space, state)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, "INSERT INTO role_space_lifecycle VALUES ($1, NULL)", space)
			require.NoError(t, err)
			called := false
			require.NoError(t, s.WithinSpaces(ctx, []uuid.UUID{space}, func(*RoleStore) error { called = true; return nil }))
			require.True(t, called)
		}
	})

	t.Run("callback error is preserved and write rolled back", func(t *testing.T) {
		marker := uuid.New()
		sentinel := errors.New("ordinary callback failure")
		err := s.WithinSpaces(ctx, []uuid.UUID{uuid.New()}, func(scoped *RoleStore) error {
			_, e := scoped.db().Exec(ctx, "INSERT INTO ordinary_scope_probe VALUES ($1)", marker)
			if e != nil {
				return e
			}
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)
		require.NotErrorIs(t, err, ErrScopeUnavailable)
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM ordinary_scope_probe WHERE id=$1", marker).Scan(&count))
		require.Zero(t, count)
	})

	t.Run("ordinary waiter sees prepared state committed under shared space lock", func(t *testing.T) {
		space := uuid.New()
		blocker, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = blocker.Rollback(context.Background()) }()
		_, err = blocker.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", space.String())
		require.NoError(t, err)
		_, err = blocker.Exec(ctx, "INSERT INTO ownership_transfer_v2 VALUES ($1, 'prepared')", space)
		require.NoError(t, err)
		done := make(chan error, 1)
		called := make(chan struct{}, 1)
		go func() {
			done <- s.WithinSpaces(ctx, []uuid.UUID{space}, func(*RoleStore) error { called <- struct{}{}; return nil })
		}()
		waitForScopeLockWaiter(t, ctx, pool)
		require.Empty(t, called)
		require.NoError(t, blocker.Commit(ctx))
		require.ErrorIs(t, awaitScopeResult(t, ctx, done), ErrSpaceFrozen)
		require.Empty(t, called)
	})

	t.Run("trusted nested scope cannot bypass a newly prepared fence", func(t *testing.T) {
		space := uuid.New()
		require.NoError(t, s.withLockedTransaction(ctx, uuid.New(), []uuid.UUID{space}, func(trusted *RoleStore) error {
			_, err := trusted.db().Exec(ctx, "INSERT INTO ownership_transfer_v2 VALUES ($1, 'prepared')", space)
			if err != nil {
				return err
			}
			called := false
			err = trusted.WithinSpaces(ctx, []uuid.UUID{space}, func(*RoleStore) error { called = true; return nil })
			require.ErrorIs(t, err, ErrSpaceFrozen)
			require.False(t, called)
			return nil
		}))
	})

	t.Run("failed fence lookup never permits callback", func(t *testing.T) {
		for _, table := range []string{"ownership_transfer_v2", "role_space_lifecycle"} {
			t.Run(table, func(t *testing.T) {
				// Static table names only. Restore the fixture even when an assertion fails.
				_, err := pool.Exec(ctx, "ALTER TABLE "+table+" RENAME TO missing_fence_fixture")
				require.NoError(t, err)
				defer func() {
					_, restoreErr := pool.Exec(ctx, "ALTER TABLE missing_fence_fixture RENAME TO "+table)
					require.NoError(t, restoreErr)
				}()
				called := false
				err = s.WithinSpaces(ctx, []uuid.UUID{uuid.New()}, func(*RoleStore) error { called = true; return nil })
				require.ErrorIs(t, err, ErrScopeUnavailable)
				require.False(t, called)
			})
		}
	})
}

func TestWithinSpacesInvalidScopeFailsClosed(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		store  *RoleStore
		spaces []uuid.UUID
	}{
		{"nil store", nil, []uuid.UUID{uuid.New()}},
		{"missing pool", &RoleStore{}, []uuid.UUID{uuid.New()}},
		{"empty scope", &RoleStore{}, nil},
		{"nil space", &RoleStore{}, []uuid.UUID{uuid.Nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			err := tc.store.WithinSpaces(ctx, tc.spaces, func(*RoleStore) error { called = true; return nil })
			require.ErrorIs(t, err, ErrScopeUnavailable)
			require.False(t, called)
		})
	}
	require.ErrorIs(t, (&RoleStore{}).WithinSpaces(ctx, []uuid.UUID{uuid.New()}, nil), ErrScopeUnavailable)
}
