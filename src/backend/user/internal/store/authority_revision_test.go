package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestUserSourceActualMigrationPreservesAuthorityFences(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and pinned migration driver")
	}
	ctx := context.Background()
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(userModuleRepoRoot(t), "src", "backend", "migrations", "user_db"))
	fixture.Run(true, "up")
	pool := fixture.Pool
	var version int64
	var dirty bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 19, version)
	require.False(t, dirty)
	readRevision := func() int64 {
		var value int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM user_authority_revision WHERE singleton`).Scan(&value))
		return value
	}
	require.EqualValues(t, 1, readRevision())
	account, profile, actor := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'source','0001','source')`, profile, account)
	require.NoError(t, err)
	first := readRevision()
	require.Greater(t, first, int64(1))
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE profiles SET display_name='rollback' WHERE id=$1`, profile)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	require.Equal(t, first, readRevision(), "rolled-back authority never changes the committed clock")
	_, err = pool.Exec(ctx, `UPDATE profiles SET frozen_at=now() WHERE id=$1`, profile)
	require.NoError(t, err)
	second := readRevision()
	require.Greater(t, second, first)
	_, err = pool.Exec(ctx, `INSERT INTO user_account_lifecycle(account_id,state,source_event_id,occurred_at) VALUES($1,'ACCOUNT_INACTIVE',$2,now())`, account, uuid.New())
	require.NoError(t, err)
	third := readRevision()
	require.Greater(t, third, second)
	_, err = pool.Exec(ctx, `INSERT INTO sdk_author_tombstones(operation_id,receipt_id,source_account_id,source_actor_id,target_account_id,target_profile_id,profile_revision,frozen_binding_id,frozen_authority_epoch,freeze_receipt_id,request_hash) VALUES($1,$2,$3,$4,$3,$5,2,$6,1,$7,repeat('0',64))`, uuid.New(), uuid.New(), account, actor, profile, uuid.New(), uuid.New())
	require.NoError(t, err)
	fourth := readRevision()
	require.Greater(t, fourth, third)
	for _, sql := range []string{
		`UPDATE user_account_lifecycle SET occurred_at=now()`, `DELETE FROM user_account_lifecycle`, `TRUNCATE user_account_lifecycle`,
		`DELETE FROM user_authority_revision`, `TRUNCATE user_authority_revision`, `UPDATE user_authority_revision SET revision=revision-1`,
		`TRUNCATE profiles CASCADE`, `TRUNCATE sdk_author_tombstones`,
	} {
		_, err = pool.Exec(ctx, sql)
		require.Error(t, err, sql)
		require.Equal(t, fourth, readRevision(), "failed writes preserve rows and counter")
	}
	_, err = pool.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.NoError(t, err)
	require.Greater(t, readRevision(), fourth)
	fixture.Run(false, "down", "1")
	require.NoError(t, pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 18, version)
	require.True(t, dirty)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM user_account_lifecycle`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM sdk_author_tombstones`).Scan(&count))
	require.Equal(t, 1, count)
	require.Positive(t, readRevision())
}
