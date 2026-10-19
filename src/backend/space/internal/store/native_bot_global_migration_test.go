package store

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// This first storage oracle does not claim an enabled native46 writer. It
// exercises the real forward migration and its protected terminal predicate;
// the signed Add/global-removal integration follows the exact46 qualification.
func TestNativeBotGlobal46StorageAndUnactivatedDown(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL forward native46 migration")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "45")
	var retained []byte
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT catalog_sha256 FROM native_membership_guard.epoch`).Scan(&retained))
	db.Run(true, "up")
	var unchanged []byte
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT catalog_sha256 FROM native_membership_guard.epoch`).Scan(&unchanged))
	require.Equal(t, retained, unchanged, "UP must not promote/reseal an established guard epoch")
	bot := uuid.New()
	var terminal bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT public.space_bot_native_terminal($1)`, bot).Scan(&terminal))
	require.False(t, terminal)
	db.Run(true, "down", "1")
	var version int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version))
	require.EqualValues(t, 45, version)
	db.Run(true, "up")
	root, actor := uuid.New(), uuid.New()
	rootSHA, actorBinding := sha256.Sum256(root[:]), sha256.Sum256(actor[:])
	_, err := db.Pool.Exec(ctx, `INSERT INTO native_membership_guard.bot_terminals(bot_id,decision_id,decision_sha256,actor_binding_sha256,created_at) VALUES($1,$2,$3,$4,clock_timestamp())`, bot, root, rootSHA[:], actorBinding[:])
	require.NoError(t, err, "trusted installer fixture records only an opaque root/actor conflict binding")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT public.space_bot_native_terminal($1)`, bot).Scan(&terminal))
	require.True(t, terminal)
	_, err = db.Pool.Exec(ctx, `UPDATE native_membership_guard.bot_terminals SET decision_id=$2 WHERE bot_id=$1`, bot, uuid.New())
	require.Error(t, err, "a completed global root cannot be rebound")
	_, err = db.Pool.Exec(ctx, `DELETE FROM native_membership_guard.bot_terminals WHERE bot_id=$1`, bot)
	require.Error(t, err, "global native revocation survives ordinary cleanup")
	_, err = db.Pool.Exec(ctx, `TRUNCATE native_membership_guard.bot_terminals`)
	require.Error(t, err)
	db.Run(false, "down", "1")
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminals WHERE bot_id=$1`, bot).Scan(&count))
	require.Equal(t, 1, count, "retained revocation forbids destructive rollback")
}

func TestNativeBotGlobal46TerminalSerializesWithAddPredicate(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL shared global Bot lock")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	bot, root, actor := uuid.New(), uuid.New(), uuid.New()
	rootSHA, actorSHA := sha256.Sum256(root[:]), sha256.Sum256(actor[:])
	add, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer add.Rollback(context.Background())
	var terminal bool
	require.NoError(t, add.QueryRow(ctx, `SELECT public.space_bot_native_terminal($1)`, bot).Scan(&terminal))
	require.False(t, terminal)
	remover, err := db.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer remover.Release()
	var pid int32
	require.NoError(t, remover.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	go func() {
		_, e := remover.Exec(ctx, `INSERT INTO native_membership_guard.bot_terminals(bot_id,decision_id,decision_sha256,actor_binding_sha256,created_at) VALUES($1,$2,$3,$4,clock_timestamp())`, bot, root, rootSHA[:], actorSHA[:])
		done <- e
	}()
	blocked := false
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		require.NoError(t, add.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked))
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Settle both owned operations even on the causal RED, avoiding a leaked
	// blocking connection or a test timeout masquerading as the intended result.
	require.NoError(t, add.Commit(ctx))
	require.NoError(t, <-done)
	require.True(t, blocked, "global terminal INSERT must share the Add predicate lock, not race its earlier absent observation")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT public.space_bot_native_terminal($1)`, bot).Scan(&terminal))
	require.True(t, terminal)
}
