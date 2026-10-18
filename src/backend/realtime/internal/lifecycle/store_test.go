//go:build realtimep3integration

package lifecycle

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("REALTIME_P3_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("REALTIME_P3_TEST_DATABASE_URL is required; run this suite in the isolated Linux PG runner")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "p3_" + uuid.New().String()[:8]
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	raw, err := os.ReadFile("../../../migrations/realtime_db/000001_admission.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	return &Store{Pool: pool}, pool
}

func TestSchemaReadinessChecksActualGatesWithoutCreatingDomainState(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	if err := s.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	var records int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM realtime_incarnations)+(SELECT count(*) FROM realtime_admissions)+(SELECT count(*) FROM realtime_lifecycle_operations)`).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if records != 0 {
		t.Fatal("readiness manufactured domain state")
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE realtime_chat_gates RENAME TO unavailable_chat_gates`); err != nil {
		t.Fatal(err)
	}
	if err := s.Ready(ctx); err == nil {
		t.Fatal("metadata schema loss advertised ready")
	}
}

// The regression exercises the decisive cut in TT c2: a process has durable
// admission, loses its database session, but can still execute a local sink.
func TestCommittedAdmissionSurvivesBackendLossUntilSinkSettles(t *testing.T) {
	s, pool := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	incarnation, chat, space, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := s.Open(ctx, incarnation, "test-container/boot/start"); err != nil {
		t.Fatal(err)
	}
	if err := s.Hydrate(ctx, chat, space, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	admission, err := s.Begin(ctx, incarnation, chat)
	if err != nil {
		t.Fatal(err)
	}
	// MaxConns=1 makes this exactly the backend that committed Begin, not an
	// unrelated idle connection. Kill it from a separate administrative session.
	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	backend := connection.Conn().PgConn().PID()
	killer, err := pgx.Connect(ctx, os.Getenv("REALTIME_P3_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = killer.Close(context.Background()) }()
	if _, err = killer.Exec(ctx, "SELECT pg_terminate_backend($1)", backend); err != nil {
		t.Fatal(err)
	}
	// Discard the deliberately killed connection rather than handing the next
	// independent operation a socket with an unread server termination error.
	_ = connection.Conn().Close(ctx)
	connection.Release()
	if err = s.Restrict(ctx, chat, space, operation, 1, "FROZEN"); err != nil {
		t.Fatal(err)
	}
	if err = s.Drained(ctx, []uuid.UUID{chat}); !errors.Is(err, ErrPending) {
		t.Fatalf("freeze receipt must remain pending, got %v", err)
	}
	if _, err = s.Begin(ctx, incarnation, chat); !errors.Is(err, ErrDenied) {
		t.Fatalf("closed gate admits new send: %v", err)
	}
	sink := make(chan string, 1)
	sink <- "previously admitted synchronous enqueue"
	if err = s.Settle(ctx, incarnation, chat, admission); err != nil {
		t.Fatal(err)
	}
	if err = s.Drained(ctx, []uuid.UUID{chat}); err != nil {
		t.Fatal(err)
	}
	if got := <-sink; got == "" {
		t.Fatal("sink did not execute")
	}
}

func TestRestartDoesNotSettleOldAdmissionAndHydrationCannotReopen(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	chat, space, old, fresh, op := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := s.Open(ctx, old, "old/boot/start"); err != nil {
		t.Fatal(err)
	}
	if err := s.Hydrate(ctx, chat, space, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	ticket, err := s.Begin(ctx, old, chat)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Open(ctx, fresh, "new/boot/start"); err != nil {
		t.Fatal(err)
	}
	if err = s.Restrict(ctx, chat, space, op, 1, "FROZEN"); err != nil {
		t.Fatal(err)
	}
	if err = s.Hydrate(ctx, chat, space, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Begin(ctx, fresh, chat); !errors.Is(err, ErrDenied) {
		t.Fatalf("stale hydration reopened gate: %v", err)
	}
	if err = s.Settle(ctx, fresh, chat, ticket); !errors.Is(err, ErrConflict) {
		t.Fatalf("new incarnation settled old ticket: %v", err)
	}
	if err = s.Drained(ctx, []uuid.UUID{chat}); !errors.Is(err, ErrPending) {
		t.Fatal(err)
	}
	if err = s.Settle(ctx, old, chat, ticket); err != nil {
		t.Fatal(err)
	}
	if err = s.Drained(ctx, []uuid.UUID{chat}); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownAndConflictingScopeDenyWhileUnrelatedLiveChatWorks(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	inc, chat, other, space := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := s.Open(ctx, inc, "test/boot/start"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Begin(ctx, inc, chat); !errors.Is(err, ErrDenied) {
		t.Fatalf("unknown gate allowed: %v", err)
	}
	if err := s.Hydrate(ctx, chat, space, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	if err := s.Hydrate(ctx, chat, uuid.New(), 0, "LIVE"); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed scope accepted: %v", err)
	}
	if err := s.Hydrate(ctx, other, uuid.Nil, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	if err := s.Restrict(ctx, chat, space, uuid.New(), 1, "PURGED"); err != nil {
		t.Fatal(err)
	}
	ticket, err := s.Begin(ctx, inc, other)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Settle(ctx, inc, other, ticket); err != nil {
		t.Fatal(err)
	}
	if err = s.Hydrate(ctx, chat, space, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Begin(ctx, inc, chat); !errors.Is(err, ErrDenied) {
		t.Fatalf("PURGED reopened: %v", err)
	}
}
