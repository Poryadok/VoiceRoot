package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is a destructive-DDL transaction oracle over the actual ledger DDL,
// not a membership or P3 provider fixture. It defines no native predicate.
func TestEffectiveEpisodeDownCannotEraseFirstCommittedWriter(t *testing.T) {
	runEffectiveEpisodeDownOracle(t, false)
}

func TestEffectiveEpisodeDownRejectsInheritedSnapshot(t *testing.T) {
	runEffectiveEpisodeDownOracle(t, true)
}

func runEffectiveEpisodeDownOracle(t *testing.T, inheritedSnapshot bool) {
	t.Helper()
	dsn := os.Getenv("VOICE_EFFECTIVE_EPISODE_TEST_DSN")
	if dsn == "" {
		t.Skip("explicit private PostgreSQL fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	database := "ledger_down_" + uuid.New().String()[:8]
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{database}.Sanitize())
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.Database = database
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	root := os.Getenv("VOICE_EFFECTIVE_EPISODE_MIGRATIONS")
	if root == "" {
		t.Fatal("actual cohort migration directory required")
	}
	paths, e := filepath.Glob(filepath.Join(root, "*.up.sql"))
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range paths {
		if filepath.Base(path) >= "000041_" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	up, e := os.ReadFile(os.Getenv("VOICE_EFFECTIVE_EPISODE_TEST_UP"))
	if e != nil {
		t.Fatal(e)
	}
	down, e := os.ReadFile(os.Getenv("VOICE_EFFECTIVE_EPISODE_TEST_DOWN"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, string(up)); e != nil {
		t.Fatal(e)
	}
	space, profile, cutover := uuid.New(), uuid.New(), uuid.New()
	if _, e = pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'private DDL oracle',$2)`, space, profile); e != nil {
		t.Fatal(e)
	}
	if inheritedSnapshot {
		remover, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		if err != nil {
			t.Fatal(err)
		}
		defer remover.Rollback(context.Background())
		var before int
		if err = remover.QueryRow(ctx, `SELECT count(*) FROM space_effective_member_heads`).Scan(&before); err != nil || before != 0 {
			t.Fatalf("empty inherited snapshot: %d %v", before, err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO space_effective_member_heads(space_id,profile_id,generation,retired_through,ledger_revision,episode_id,state,boundary_state,observed_at,cutover_id,ordinary_legacy) VALUES($1,$2,0,0,1,NULL,'NONE','UNKNOWN',clock_timestamp(),$3,false)`, space, profile, cutover); err != nil {
			t.Fatal(err)
		}
		_, err = remover.Exec(ctx, string(down))
		_ = remover.Rollback(ctx)
		pgErr, ok := err.(*pgconn.PgError)
		if !ok || pgErr.Code != "55000" {
			t.Fatalf("DOWN must reject inherited snapshot before DROP: %v", err)
		}
		var retained bool
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2)`, space, profile).Scan(&retained); err != nil || !retained {
			t.Fatalf("committed row not preserved after inherited snapshot: %v", err)
		}
		return
	}
	writer, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Rollback(context.Background())
	if _, e = writer.Exec(ctx, `SELECT id FROM spaces WHERE id=$1 FOR KEY SHARE`, space); e != nil {
		t.Fatal(e)
	}
	if _, e = writer.Exec(ctx, `LOCK TABLE space_effective_member_heads IN ROW EXCLUSIVE MODE`); e != nil {
		t.Fatal(e)
	}
	remover, e := pool.Acquire(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer remover.Release()
	var pid int32
	if e = remover.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() {
		_, err := remover.Exec(ctx, string(down))
		_, _ = remover.Exec(context.Background(), `ROLLBACK`)
		done <- err
	}()
	blocked := false
	for i := 0; i < 200; i++ {
		if e = writer.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked); e != nil {
			t.Fatal(e)
		}
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("DOWN did not reach the actual table lock")
	}
	if _, e = writer.Exec(ctx, `INSERT INTO space_effective_member_heads(space_id,profile_id,generation,retired_through,ledger_revision,episode_id,state,boundary_state,history_visible_since,covered_until,observed_at,cutover_id,ordinary_witness,ordinary_legacy)
 VALUES($1,$2,0,0,1,NULL,'NONE','UNKNOWN',NULL,NULL,clock_timestamp(),$3,NULL,false)`, space, profile, cutover); e != nil {
		t.Fatal(e)
	}
	if e = writer.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = <-done; e == nil {
		t.Fatal("DOWN erased the first committed private ledger row")
	}
	var retained bool
	if e = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='NONE')`, space, profile).Scan(&retained); e != nil || !retained {
		t.Fatalf("committed row not preserved: %v", e)
	}
}
