//go:build realtimep3integration

package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestRealtimeParticipantElevenSQLCannotHardDeleteWithTen(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("REALTIME_P3_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned isolated PostgreSQL is required")
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	database := "spacep3_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{database}.Sanitize())
		require.NoError(t, err)
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	paths, err := filepath.Glob("../../../migrations/space_db/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err, path)
	}
	space, operation, manifest, profile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'Realtime barrier',$2)`, space, profile)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO space_lifecycle_aggregates(space_id,deletion_operation_id,phase,generation,manifest_id,manifest_sha256,manifest_item_count,scheduled_at,purge_after,purge_decided_at,completed_at,local_purge_completed) VALUES($1,$2,'PURGED',2,$3,decode(repeat('ab',32),'hex'),0,now()-interval '2 hours',now()-interval '1 hour',now()-interval '1 hour',now(),true)`, space, operation, manifest)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO space_deletion_tombstones(space_id,owner_account_hmac,actor_account_hmac,key_version,reason,scheduled_at,purge_after,purge_decided_at,purged_at,retain_until) VALUES($1,decode(repeat('ab',32),'hex'),decode(repeat('cd',32),'hex'),'test','OWNER_REQUESTED',now()-interval '2 hours',now()-interval '1 hour',now()-interval '1 hour',now(),now()+interval '365 days')`, space)
	require.NoError(t, err)
	insert := func(id int) {
		_, err = pool.Exec(ctx, `INSERT INTO space_lifecycle_participants(space_id,deletion_operation_id,generation,participant_id,request_kind,progress,request_bytes,request_sha256,receipt_bytes,receipt_sha256,manifest_id,manifest_sha256,manifest_item_count,completed_at) VALUES($1,$2,2,$3::smallint,CASE WHEN $3::smallint=1 THEN 'ROLE_RETIREMENT' ELSE 'PURGE' END,'COMPLETE','x',decode(repeat('ab',32),'hex'),'y',decode(repeat('cd',32),'hex'),$4,decode(repeat('ab',32),'hex'),0,now())`, space, operation, id, manifest)
		require.NoError(t, err)
	}
	for id := 1; id <= 10; id++ {
		insert(id)
	}
	var ready bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT space_terminal_purge_ready($1)`, space).Scan(&ready))
	require.False(t, ready)
	remove := func() error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `SET LOCAL voice.audit_delete_mode='space_purge'`); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM spaces WHERE id=$1`, space); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	require.Error(t, remove(), "ten historical participants cannot hard-delete Space")
	insert(11)
	require.NoError(t, pool.QueryRow(ctx, `SELECT space_terminal_purge_ready($1)`, space).Scan(&ready))
	require.True(t, ready)
	_, err = pool.Exec(ctx, `INSERT INTO space_lifecycle_completions(space_id,deletion_operation_id,generation,participant_id,request_kind,request_sha256,receipt_sha256,manifest_sha256,completed_at) VALUES($1,$2,2,11,'PURGE',decode(repeat('ab',32),'hex'),decode(repeat('cd',32),'hex'),decode(repeat('ab',32),'hex'),now())`, space, operation)
	require.NoError(t, err)
	require.NoError(t, remove())
	var retained int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_deletion_tombstones WHERE space_id=$1`, space).Scan(&retained))
	require.Equal(t, 1, retained)
	raw, err := os.ReadFile("../../../migrations/space_db/000034_realtime_lifecycle_participant.down.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(raw))
	require.Error(t, err, "rollback must not reintroduce an old ten-participant writer")
}
