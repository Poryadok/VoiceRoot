package ordinarycall

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func ordinaryMigration(t *testing.T, direction string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000012_ordinary_calls."+direction+".sql"))
	require.NoError(t, err)
	return string(b)
}

func insertDownEvidence(t *testing.T, ctx context.Context, tx pgx.Tx, kind string) (table string) {
	t.Helper()
	if kind == "journal" {
		r, _, _ := pendingGroup(t)
		body, err := json.Marshal(r)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `INSERT INTO voice_ordinary_rooms(room_id,operation_id,chat_id,is_group,state,version,document)
VALUES($1,$2,$3,true,'PREPARING',1,$4)`, r.Intent.RoomID, r.Intent.OperationID, r.Intent.ChatID, body)
		require.NoError(t, err)
		return "voice_ordinary_rooms"
	}
	_, err := tx.Exec(ctx, `INSERT INTO voice_sdk_conversion_fence_receipts(operation_id,binding_id,source_account_id,source_actor_id,
source_profile_id,target_account_id,target_profile_id,frozen_authority_epoch,freeze_receipt_id,request_hash,receipt_id,state,ordinary_source_snapshot)
VALUES($1,$2,$3,$4,$5,$6,$7,1,$8,$9,$10,'pending','{"Version":1}')`, uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), make([]byte, 32), uuid.New())
	require.NoError(t, err)
	return "voice_sdk_conversion_fence_receipts"
}

func TestOrdinaryDownRetainsConcurrentFirstWriter(t *testing.T) {
	for _, kind := range []string{"journal", "snapshot"} {
		t.Run(kind, func(t *testing.T) {
			pool := ordinaryPostgres(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			writer, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = writer.Rollback(context.Background()) }()
			table := insertDownEvidence(t, ctx, writer, kind)
			var expected string
			require.NoError(t, writer.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(e))::text FROM `+table+` e`).Scan(&expected))
			executor, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer executor.Release()
			var pid int
			require.NoError(t, executor.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
			down := ordinaryMigration(t, "down")
			done := make(chan error, 1)
			go func() { _, err := executor.Exec(ctx, down); done <- err }()
			require.Eventually(t, func() bool {
				var waiting bool
				err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1
AND relation=$2::regclass AND mode='AccessExclusiveLock' AND NOT granted)`, pid, table).Scan(&waiting)
				return err == nil && waiting
			}, 5*time.Second, 10*time.Millisecond, "actual DOWN waits on the real uncommitted writer")
			require.NoError(t, writer.Commit(ctx))
			select {
			case err = <-done:
			case <-ctx.Done():
				t.Fatal("DOWN did not finish after writer commit")
			}
			_, _ = executor.Exec(context.Background(), "ROLLBACK")
			require.ErrorContains(t, err, "explicit retirement protocol")
			var actual string
			require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(e))::text FROM `+table+` e`).Scan(&actual))
			require.JSONEq(t, expected, actual, "all committed evidence survives refused DOWN")
		})
	}
}

func TestOrdinaryDownRejectsInheritedSnapshot(t *testing.T) {
	pool := ordinaryPostgres(t)
	ctx := context.Background()
	executor, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer func() { _ = executor.Rollback(ctx) }()
	var snapshot string
	require.NoError(t, executor.QueryRow(ctx, `SELECT txid_current_snapshot()::text`).Scan(&snapshot))
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	table := insertDownEvidence(t, ctx, writer, "journal")
	require.NoError(t, writer.Commit(ctx))
	_, err = executor.Exec(ctx, ordinaryMigration(t, "down"))
	_ = executor.Rollback(ctx)
	require.ErrorContains(t, err, "READ COMMITTED")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count))
	require.Equal(t, 1, count)
}

func TestOrdinaryEmptyDownUp(t *testing.T) {
	pool := ordinaryPostgres(t)
	ctx := context.Background()
	for _, direction := range []string{"down", "up"} {
		_, err := pool.Exec(ctx, ordinaryMigration(t, direction))
		require.NoError(t, err)
	}
	require.NoError(t, NewPostgresStore(pool, storageTestEncoder).CheckSchema(ctx))
}
