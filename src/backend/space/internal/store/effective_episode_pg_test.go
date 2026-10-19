package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Explicit fixture: this suite verifies the new compact ledger SQL/CAS, not
// Testcontainers provisioning, native publication proofs or activated writers.
func TestEffectiveEpisodePostgres(t *testing.T) {
	dsn := os.Getenv("VOICE_EFFECTIVE_EPISODE_TEST_DSN")
	if dsn == "" {
		t.Skip("explicit isolated ledger PostgreSQL fixture not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	database := "ledger_cas_" + uuid.New().String()[:8]
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{database}.Sanitize())
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sql, err := os.ReadFile(os.Getenv("VOICE_EFFECTIVE_EPISODE_TEST_UP"))
	if err != nil {
		t.Fatal(err)
	}
	// Install the actual native39 provider and canonical eleven-participant
	// predicate. This private CAS suite still does not authorize native sources;
	// the separate real finalizer test covers positive P3 minimization.
	root := os.Getenv("VOICE_EFFECTIVE_EPISODE_MIGRATIONS")
	if root == "" {
		t.Fatal("actual cohort migration directory required")
	}
	paths, err := filepath.Glob(filepath.Join(root, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if filepath.Base(path) >= "000041_" {
			continue
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(raw)); e != nil {
			t.Fatalf("%s: %v", path, e)
		}
	}
	if _, err = pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time {
		var at time.Time
		if e := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); e != nil {
			t.Fatal(e)
		}
		return at
	}
	begin := func(tb testing.TB) pgx.Tx {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		tb.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		return tx
	}
	space, profile, cutover := uuid.New(), uuid.New(), uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'private CAS oracle',$2)`, space, profile); err != nil {
		t.Fatal(err)
	}
	at := clock()
	source := effectiveSources{Ordinary: "native-ordinary-generation-publication"}
	head, err := advanceEffectiveEpisode(effectiveHead{}, effectiveSources{}, source, at, at, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	tx := begin(t)
	if err = saveEffectiveHead(ctx, tx, space, profile, cutover, 0, head); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Run("roundtrip exact witness and infinite ordinary horizon", func(t *testing.T) {
		tx := begin(t)
		defer tx.Rollback(ctx)
		got, c, e := loadEffectiveHead(ctx, tx, space, profile)
		if e != nil || c != cutover || got.EpisodeID != head.EpisodeID || !got.Sources.equal(head.Sources) || !got.Since.Equal(head.Since) {
			t.Fatalf("roundtrip: %+v %v", got, e)
		}
	})
	t.Run("missing head has no read repair", func(t *testing.T) {
		tx := begin(t)
		defer tx.Rollback(ctx)
		_, _, e := loadEffectiveHead(ctx, tx, space, uuid.New())
		if !errors.Is(e, ErrEffectiveEpisodeUnavailable) {
			t.Fatal(e)
		}
		var count int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM space_effective_member_heads`).Scan(&count); e != nil || count != 1 {
			t.Fatalf("count %d %v", count, e)
		}
	})
	t.Run("rollback and stale CAS do not alter durable episode", func(t *testing.T) {
		now := clock()
		next, e := advanceEffectiveEpisode(head, source, effectiveSources{}, now, now, uuid.New())
		if e != nil {
			t.Fatal(e)
		}
		tx := begin(t)
		if e = saveEffectiveHead(ctx, tx, space, profile, cutover, head.Revision, next); e != nil {
			t.Fatal(e)
		}
		_ = tx.Rollback(ctx)
		tx = begin(t)
		got, _, e := loadEffectiveHead(ctx, tx, space, profile)
		if e != nil || got.Revision != head.Revision || !got.Active {
			t.Fatalf("rollback leaked: %+v %v", got, e)
		}
		_ = tx.Rollback(ctx)
		tx = begin(t)
		if e = saveEffectiveHead(ctx, tx, space, profile, uuid.New(), head.Revision, next); !errors.Is(e, ErrEffectiveEpisodeUnavailable) {
			t.Fatalf("foreign cutover %v", e)
		}
	})
	t.Run("inherited repeatable read is rejected", func(t *testing.T) {
		tx, e := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if e = saveEffectiveHead(ctx, tx, uuid.New(), uuid.New(), cutover, 0, head); !errors.Is(e, ErrEffectiveEpisodeUnavailable) {
			t.Fatal(e)
		}
	})
	t.Run("SQL guards reject rebind truncate and retained DOWN", func(t *testing.T) {
		down, e := os.ReadFile(os.Getenv("VOICE_EFFECTIVE_EPISODE_TEST_DOWN"))
		if e != nil {
			t.Fatal(e)
		}
		for _, query := range []string{`UPDATE space_effective_member_heads SET episode_id=gen_random_uuid(),ledger_revision=ledger_revision+1`, `TRUNCATE space_effective_member_heads`, string(down)} {
			tx := begin(t)
			if _, e = tx.Exec(ctx, query); e == nil {
				t.Fatalf("guard accepted %s", query)
			}
			_ = tx.Rollback(ctx)
		}
	})
	t.Run("SQL wait crossing final lease rolls back", func(t *testing.T) {
		now := clock()
		s := effectiveSources{Community: "native-community-owner-generation-episode", CommunityUntil: now.Add(1500 * time.Millisecond)}
		h, e := advanceEffectiveEpisode(effectiveHead{}, effectiveSources{}, s, now, now, uuid.New())
		if e != nil {
			t.Fatal(e)
		}
		p := uuid.New()
		tx := begin(t)
		if e = saveEffectiveHead(ctx, tx, space, p, cutover, 0, h); e != nil {
			t.Fatal(e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		holder := begin(t)
		if _, _, e = loadEffectiveHead(ctx, holder, space, p); e != nil {
			t.Fatal(e)
		}
		worker := begin(t)
		var pid int32
		if e = worker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); e != nil {
			t.Fatal(e)
		}
		now = clock()
		renewed := s
		renewed.CommunityUntil = now.Add(time.Minute)
		next, e := advanceEffectiveEpisode(h, s, renewed, now, now, uuid.New())
		if e != nil {
			t.Fatal(e)
		}
		done := make(chan error, 1)
		go func() { done <- saveEffectiveHead(ctx, worker, space, p, cutover, h.Revision, next) }()
		blocked := false
		for i := 0; i < 100; i++ {
			if e = holder.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked); e != nil {
				t.Fatal(e)
			}
			if blocked {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !blocked {
			t.Fatal("worker never reached actual row lock")
		}
		if _, e = holder.Exec(ctx, `SELECT pg_sleep(1.6)`); e != nil {
			t.Fatal(e)
		}
		if e = holder.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		if e = <-done; !errors.Is(e, ErrEffectiveEpisodeUnavailable) {
			t.Fatalf("expired final support accepted: %v", e)
		}
		_ = worker.Rollback(ctx)
		tx = begin(t)
		got, _, e := loadEffectiveHead(ctx, tx, space, p)
		if e != nil || got.Revision != h.Revision || got.EpisodeID != h.EpisodeID {
			t.Fatalf("failed CAS escaped rollback: %+v %v", got, e)
		}
	})
}
