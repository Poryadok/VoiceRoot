package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	commonv1 "voice.app/voice/common/v1"
)

func TestReportTargetOwnerPostgres(t *testing.T) {
	dsn := os.Getenv("REPORT_TARGET_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated owner PostgreSQL fixture required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	namespace := "report_target_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+namespace); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+namespace+" CASCADE") }()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `CREATE TABLE messages(id uuid PRIMARY KEY,deleted_at timestamptz);`); err != nil {
		t.Fatal(err)
	}
	s := &MessagesStore{Pool: pool}
	id := uuid.New()
	state, err := s.ResolveReportTarget(ctx, id)
	if err != nil || state != commonv1.ReportTargetState_REPORT_TARGET_STATE_MISSING {
		t.Fatalf("missing: %v %v", state, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO messages(id) VALUES($1)`, id)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.ResolveReportTarget(ctx, id)
	if err != nil || state != commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE {
		t.Fatalf("live: %v %v", state, err)
	}

	_, err = pool.Exec(ctx, `UPDATE messages SET deleted_at=now() WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.ResolveReportTarget(ctx, id)
	if err != nil || state != commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED {
		t.Fatalf("deleted: %v %v", state, err)
	}

}
