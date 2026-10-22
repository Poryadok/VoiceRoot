package grpcsvc

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/space/internal/store"
)

// Audit fixtures represent existing legacy membership, not a new public
// admission. Establish that real baseline before native migration, then run
// every audit assertion against the actual qualified current schema.
func nativeAuditFixture(t *testing.T, ctx context.Context, owner, member uuid.UUID, name string) (*pgxpool.Pool, *store.SpaceRow) {
	t.Helper()
	return nativeAuditFixtureWithDescription(t, ctx, owner, member, name, "private")
}

func nativeAuditFixtureWithDescription(t *testing.T, ctx context.Context, owner, member uuid.UUID, name, description string) (*pgxpool.Pool, *store.SpaceRow) {
	t.Helper()
	directory := filepath.Join(repoRoot(t), "src/backend/migrations/space_db")
	db := auditMigrationFixture(t, ctx, directory)
	db.Run(true, "goto", "34")
	st := &store.SpaceStore{Pool: db.Pool}
	row, err := st.CreateSpace(ctx, owner, name, description, "private")
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, row.ID, member)
	require.NoError(t, err)
	db.Run(true, "up")
	var version int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version))
	require.EqualValues(t, 45, version)
	_, err = st.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.NoError(t, st.InitializeNativeMembership(ctx, row.ID))
	var complete bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state='COMPLETE' FROM native_membership_guard.spaces WHERE space_id=$1`, row.ID).Scan(&complete))
	require.True(t, complete)
	// Production capture is qualified for a nonowner serving LOGIN. The trusted
	// installer pool is retained only by fixture cleanup, never returned as an
	// application writer.
	role := "audit_serve_" + uuid.New().String()[:8]
	_, err = db.Pool.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD 'isolated-audit-serving'")
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{role}.Sanitize()+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	config := db.Pool.Config().Copy()
	config.ConnConfig.User, config.ConnConfig.Password = role, "isolated-audit-serving"
	serving, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		serving.Close()
		_, _ = db.Pool.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		_, _ = db.Pool.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	return serving, row
}

func TestNativeAuditScheduleCaptureAndExactRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	owner, member, account := uuid.New(), uuid.New(), uuid.New()
	pool, row := nativeAuditFixture(t, ctx, owner, member, "audit schedule")
	st := &store.SpaceStore{Pool: pool}
	_, err := st.CreateSpace(ctx, owner, "unframed must stay denied", "", "private")
	var guarded *pgconn.PgError
	require.ErrorAs(t, err, &guarded)
	require.Equal(t, "55000", guarded.Code, "fixture repair must not disable native enforcement")
	request := &spacev1.DeleteSpaceRequest{SpaceId: row.ID.String(), OperationId: uuid.NewString(), ConfirmationName: row.Name, Proof: "test-owned-proof"}
	_, err = st.ReserveLifecycleSchedule(ctx, account, member, 7, request)
	require.ErrorIs(t, err, store.ErrLifecycleConflict)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_operations WHERE space_id=$1`, row.ID).Scan(&count))
	require.Zero(t, count, "an unauthorized schedule must create no operation")
	first, err := st.ReserveLifecycleSchedule(ctx, account, owner, 7, request)
	require.NoError(t, err, "authorized scheduling must complete the native DB capture")
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, first.Phase())
	again, err := st.ReserveLifecycleSchedule(ctx, account, owner, 7, request)
	require.NoError(t, err, "exact pending retry must not mutate native membership")
	require.Equal(t, first.Snapshot(), again.Snapshot())
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_operations WHERE space_id=$1`, row.ID).Scan(&count))
	require.Equal(t, 1, count)
}

// The optional native runner supplies an independently owned private database
// and the pinned migration executable. CI otherwise retains the existing real
// container/migration-driver fixture. No schema version is written by this code.
func auditMigrationFixture(t *testing.T, ctx context.Context, directory string) integrationtest.SourceMigrationFixture {
	t.Helper()
	dsn, binary := os.Getenv("VOICE_SPACE_AUDIT_FIXTURE_DSN"), os.Getenv("VOICE_SPACE_AUDIT_MIGRATE")
	if dsn == "" && binary == "" {
		return integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, directory, "postgres:18-alpine")
	}
	require.True(t, dsn != "" && binary != "", "native audit fixture requires both database and pinned migrator")
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	name := "audit_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	parsed.Path = "/" + name
	pool, err := pgxpool.New(ctx, parsed.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		require.NoError(t, err)
		admin.Close()
	})
	return integrationtest.SourceMigrationFixture{Pool: pool, Run: func(wantSuccess bool, arguments ...string) {
		t.Helper()
		args := append([]string{"-path", directory, "-database", parsed.String()}, arguments...)
		output, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
		if wantSuccess {
			require.True(t, err == nil, "real migration driver failed: %s", output)
		} else {
			require.Error(t, err)
		}
	}}
}
