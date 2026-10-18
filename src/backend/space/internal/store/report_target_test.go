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
	if _, err = pool.Exec(ctx, `CREATE TABLE spaces(id uuid PRIMARY KEY,deleted_at timestamptz);CREATE TABLE space_permanent_purge_fences(space_id uuid PRIMARY KEY);CREATE TABLE ownership_journal(space_id uuid,state text);CREATE TABLE space_lifecycle_aggregates(space_id uuid PRIMARY KEY,phase text,scheduled_at timestamptz,purge_after timestamptz);`); err != nil {
		t.Fatal(err)
	}
	s := &SpaceStore{Pool: pool}
	id := uuid.New()
	state, err := s.ResolveReportTarget(ctx, id)
	if err != nil || state != commonv1.ReportTargetState_REPORT_TARGET_STATE_MISSING {
		t.Fatalf("missing: %v %v", state, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO spaces(id) VALUES($1)`, id)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.ResolveReportTarget(ctx, id)
	if err != nil || state != commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE {
		t.Fatalf("live: %v %v", state, err)
	}

	for _, phase := range []string{"SCHEDULE_PENDING", "FREEZE_PENDING", "RESTORE_DECIDED", "UNKNOWN"} {
		_, err = pool.Exec(ctx, `INSERT INTO space_lifecycle_aggregates(space_id,phase) VALUES($1,$2) ON CONFLICT(space_id) DO UPDATE SET phase=$2`, id, phase)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ResolveReportTarget(ctx, id); err == nil {
			t.Fatalf("uncertain phase %s certified", phase)
		}
	}
	_, err = pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET phase='SCHEDULED',scheduled_at=now(),purge_after=now()+interval '7 days' WHERE space_id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.ResolveReportTarget(ctx, id)
	if err != nil || state != commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED {
		t.Fatalf("scheduled: %v %v", state, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO space_permanent_purge_fences VALUES($1)`, id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `DELETE FROM spaces WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.ResolveReportTarget(ctx, id)
	if err != nil || state != commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED {
		t.Fatalf("permanent fence: %v %v", state, err)
	}

}
