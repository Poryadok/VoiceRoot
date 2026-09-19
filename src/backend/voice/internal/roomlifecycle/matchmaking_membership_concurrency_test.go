package roomlifecycle

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchmakingMembershipMigrationReapply(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "mmreapply")
	baseline := r22VoiceSchemaSnapshot(t, ctx, pool)
	_, err := pool.Exec(ctx, mmReadMigration(t, "up"))
	require.NoError(t, err)
	expanded := r22VoiceSchemaSnapshot(t, ctx, pool)
	_, err = pool.Exec(ctx, mmReadMigration(t, "up"))
	require.NoError(t, err)
	require.Equal(t, expanded, r22VoiceSchemaSnapshot(t, ctx, pool))
	_, err = pool.Exec(ctx, mmReadMigration(t, "down"))
	require.NoError(t, err)
	require.Equal(t, baseline, r22VoiceSchemaSnapshot(t, ctx, pool))
	_, err = pool.Exec(ctx, mmReadMigration(t, "up"))
	require.NoError(t, err)
	require.Equal(t, expanded, r22VoiceSchemaSnapshot(t, ctx, pool))
}

func TestMatchmakingMembershipDownRechecksCommittedIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "mmdownrace")
	_, err := pool.Exec(ctx, mmReadMigration(t, "up"))
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	seed, err := r22TrySeedMigrationRows(ctx, tx, nil)
	require.NoError(t, err)
	require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, mmKnownIdentity))
	down, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer down.Release()
	var pid int
	require.NoError(t, down.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	result := make(chan error, 1)
	sql := mmReadMigration(t, "down")
	go func() { _, err := down.Exec(ctx, sql); result <- err }()
	r22WaitForBackendLock(t, ctx, pool, pid)
	require.NoError(t, tx.Commit(ctx))
	r22RequireSQLState(t, <-result, "55000")
	_, err = down.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	identity, found, err := NewPostgresLifecycleStore(pool).LoadMembershipIdentity(ctx, seed.membershipID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(7), identity.SessionEpoch)
}
