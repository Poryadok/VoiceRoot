package integrationtest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type previewRollbackFixture struct {
	pool   *pgxpool.Pool
	down   string
	up     string
	table  string
	column string
}

func previewRollbackSchema(t *testing.T, root *pgxpool.Pool, kind string) previewRollbackFixture {
	t.Helper()
	ctx := context.Background()
	schema := "preview_rollback_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := root.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	config := root.Config()
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	f := previewRollbackFixture{pool: pool, table: "files", column: "preview_metadata_revision"}
	file := "file_db/000007_preview_metadata"
	prerequisite := `CREATE TABLE files(id uuid PRIMARY KEY)`
	if kind == "messaging" {
		f.table, f.column = "messages", "file_preview_bindings"
		file = "messaging_db/000034_file_preview"
		prerequisite = `CREATE TABLE messages(id uuid PRIMARY KEY,deleted_at timestamptz,is_e2e boolean NOT NULL DEFAULT false); INSERT INTO messages(id) VALUES('00000000-0000-4000-8000-000000000001')`
	}
	_, err = pool.Exec(ctx, prerequisite)
	require.NoError(t, err)
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	base := filepath.Join(filepath.Dir(source), "..", "..", "migrations")
	up, err := os.ReadFile(filepath.Join(base, file+".up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(base, file+".down.sql"))
	require.NoError(t, err)
	f.up, f.down = string(up), string(down)
	_, err = pool.Exec(ctx, f.up)
	require.NoError(t, err)
	return f
}

func previewColumnPresent(t *testing.T, f previewRollbackFixture) bool {
	t.Helper()
	var present bool
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 AND column_name=$2)`, f.table, f.column).Scan(&present))
	return present
}

func TestPreviewMigrationRollbackPreservesAuthority(t *testing.T) {
	ctx := context.Background()
	root := StartPostgres(t, ctx, "preview_down_authority", "")
	for _, kind := range []string{"file", "messaging"} {
		t.Run(kind+"/pristine_down_and_reup", func(t *testing.T) {
			f := previewRollbackSchema(t, root, kind)
			_, err := f.pool.Exec(ctx, f.down)
			require.NoError(t, err)
			require.False(t, previewColumnPresent(t, f))
			_, err = f.pool.Exec(ctx, f.up)
			require.NoError(t, err)
			require.True(t, previewColumnPresent(t, f))
		})
	}
	for _, tc := range []struct{ name, kind, write, retained string }{
		{"default_revision_is_authority", "file", `INSERT INTO files(id) VALUES(gen_random_uuid())`, `SELECT count(*) FROM files WHERE preview_metadata_revision=1`},
		{"revision_only_drained_outbox", "file", `INSERT INTO files(id,preview_metadata_revision) VALUES(gen_random_uuid(),7)`, `SELECT count(*) FROM files WHERE preview_metadata_revision=7`},
		{"pending_outbox", "file", `INSERT INTO file_preview_event_outbox(event_id,file_id,metadata_revision,payload) VALUES(gen_random_uuid(),gen_random_uuid(),2,'retained')`, `SELECT count(*) FROM file_preview_event_outbox WHERE published_at IS NULL`},
		{"published_outbox", "file", `INSERT INTO file_preview_event_outbox(event_id,file_id,metadata_revision,payload,published_at) VALUES(gen_random_uuid(),gen_random_uuid(),2,'retained',clock_timestamp())`, `SELECT count(*) FROM file_preview_event_outbox WHERE published_at IS NOT NULL`},
		{"terminal_only_floor", "messaging", `INSERT INTO messaging_file_preview_snapshots VALUES('00000000-0000-4000-8000-000000000002',4,'opaque terminal',true,clock_timestamp())`, `SELECT count(*) FROM messaging_file_preview_snapshots WHERE terminal`},
		{"highest_snapshot", "messaging", `INSERT INTO messaging_file_preview_snapshots VALUES('00000000-0000-4000-8000-000000000002',3,'opaque ready',false,clock_timestamp())`, `SELECT count(*) FROM messaging_file_preview_snapshots WHERE metadata_revision=3`},
		{"private_binding_only", "messaging", `UPDATE messages SET file_preview_bindings='[{"file_id":"retained"}]'`, `SELECT count(*) FROM messages WHERE file_preview_bindings<>'[]'::jsonb`},
		{"pending_frozen_work", "messaging", `INSERT INTO messaging_file_preview_snapshots VALUES('00000000-0000-4000-8000-000000000002',4,'opaque terminal',true,clock_timestamp()); INSERT INTO messaging_file_preview_pending(file_id,chat_id) VALUES('00000000-0000-4000-8000-000000000002',gen_random_uuid())`, `SELECT count(*) FROM messaging_file_preview_pending`},
		{"migration_error", "messaging", `INSERT INTO messaging_file_preview_errors(message_id,reason) VALUES(gen_random_uuid(),'requires owner repair')`, `SELECT count(*) FROM messaging_file_preview_errors`},
		{"reconciliation_progress", "messaging", `UPDATE messaging_file_preview_reconciliation SET after_id=gen_random_uuid(),pass=1`, `SELECT count(*) FROM messaging_file_preview_reconciliation WHERE pass=1`},
		{"missing_reconciliation_seed", "messaging", `DELETE FROM messaging_file_preview_reconciliation`, `SELECT 1`},
	} {
		t.Run(tc.kind+"/"+tc.name, func(t *testing.T) {
			f := previewRollbackSchema(t, root, tc.kind)
			_, err := f.pool.Exec(ctx, tc.write)
			require.NoError(t, err)
			_, err = f.pool.Exec(ctx, f.down)
			var pgerr *pgconn.PgError
			require.ErrorAs(t, err, &pgerr, "retained authority must refuse DOWN")
			require.Equal(t, "55000", pgerr.Code)
			require.True(t, previewColumnPresent(t, f), "failed DOWN must retain the schema")
			var count int
			require.NoError(t, f.pool.QueryRow(ctx, tc.retained).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestPreviewMigrationRollbackFirstWriterCannotDisappear(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	root := StartPostgres(t, ctx, "preview_down_first_writer", "")
	for _, tc := range []struct{ name, kind, write, retained string }{
		{"file_first_revision", "file", `INSERT INTO files(id,preview_metadata_revision) VALUES(gen_random_uuid(),2)`, `SELECT count(*) FROM files WHERE preview_metadata_revision=2`},
		{"file_first_outbox", "file", `INSERT INTO file_preview_event_outbox(event_id,file_id,metadata_revision,payload) VALUES(gen_random_uuid(),gen_random_uuid(),2,'first writer')`, `SELECT count(*) FROM file_preview_event_outbox`},
		{"messaging_first_terminal", "messaging", `INSERT INTO messaging_file_preview_snapshots VALUES(gen_random_uuid(),4,'first terminal',true,clock_timestamp())`, `SELECT count(*) FROM messaging_file_preview_snapshots WHERE terminal`},
		{"messaging_first_binding", "messaging", `UPDATE messages SET file_preview_bindings='[{"file_id":"first writer"}]'`, `SELECT count(*) FROM messages WHERE file_preview_bindings<>'[]'::jsonb`},
		{"messaging_first_error", "messaging", `INSERT INTO messaging_file_preview_errors(message_id,reason) VALUES(gen_random_uuid(),'first error')`, `SELECT count(*) FROM messaging_file_preview_errors`},
		{"messaging_first_cursor", "messaging", `UPDATE messaging_file_preview_reconciliation SET pass=1`, `SELECT count(*) FROM messaging_file_preview_reconciliation WHERE pass=1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := previewRollbackSchema(t, root, tc.kind)
			writer, err := f.pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = writer.Rollback(context.Background()) }()
			_, err = writer.Exec(ctx, tc.write)
			require.NoError(t, err, "positive control: the first writer reaches actual PostgreSQL")
			conn, err := f.pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			pid := conn.Conn().PgConn().PID()
			done := make(chan error, 1)
			go func() { _, e := conn.Exec(ctx, f.down); done <- e }()
			var downErr error
			finished := false
			require.Eventually(t, func() bool {
				select {
				case downErr = <-done:
					finished = true
					return true
				default:
				}
				var waiting bool
				e := f.pool.QueryRow(ctx, `SELECT COALESCE((SELECT wait_event_type='Lock' FROM pg_stat_activity WHERE pid=$1),false)`, pid).Scan(&waiting)
				return e == nil && waiting
			}, 5*time.Second, 10*time.Millisecond, "observe actual SQL lock wait or early refusal before committing writer")
			require.NoError(t, writer.Commit(ctx), "positive control: first authority writer commits")
			if !finished {
				downErr = <-done
			}
			var pgerr *pgconn.PgError
			require.True(t, errors.As(downErr, &pgerr), "DOWN must not erase a newly committed authority record: %v", downErr)
			require.Contains(t, []string{"55P03", "55000"}, pgerr.Code)
			require.True(t, previewColumnPresent(t, f))
			var count int
			require.NoError(t, f.pool.QueryRow(ctx, tc.retained).Scan(&count))
			require.Equal(t, 1, count, "first writer evidence survives DOWN attempt")
		})
	}
}

func TestPreviewMigrationRollbackRejectsStaleRepeatableReadSnapshot(t *testing.T) {
	ctx := context.Background()
	root := StartPostgres(t, ctx, "preview_down_stale_snapshot", "")
	for _, tc := range []struct{ kind, table, write string }{
		{"file", "files", `INSERT INTO files(id,preview_metadata_revision) VALUES(gen_random_uuid(),2)`},
		{"messaging", "messaging_file_preview_snapshots", `INSERT INTO messaging_file_preview_snapshots VALUES(gen_random_uuid(),4,'retained terminal',true,clock_timestamp())`},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			f := previewRollbackSchema(t, root, tc.kind)
			stale, err := f.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
			require.NoError(t, err)
			defer func() { _ = stale.Rollback(context.Background()) }()
			var count int
			require.NoError(t, stale.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{tc.table}.Sanitize()).Scan(&count))
			require.Zero(t, count, "establish an actual empty repeatable-read snapshot")
			_, err = f.pool.Exec(ctx, tc.write)
			require.NoError(t, err, "another connection positively commits the first authority record")
			require.NoError(t, f.pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{tc.table}.Sanitize()).Scan(&count))
			require.Equal(t, 1, count)
			require.NoError(t, stale.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{tc.table}.Sanitize()).Scan(&count))
			require.Zero(t, count, "the caller still cannot see the committed authority record")
			_, err = stale.Exec(ctx, f.down)
			var pgerr *pgconn.PgError
			require.ErrorAs(t, err, &pgerr, "DOWN cannot trust a stale caller snapshot")
			require.Equal(t, "55000", pgerr.Code)
			require.NoError(t, stale.Rollback(ctx))
			require.True(t, previewColumnPresent(t, f))
			require.NoError(t, f.pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{tc.table}.Sanitize()).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}
