package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/mmadmission"
)

func TestLegacyStartupDescriptorConflictWithoutDatabase(t *testing.T) {
	for _, observation := range []error{nil, os.ErrPermission, errors.New("descriptor observation unavailable")} {
		require.ErrorIs(t, checkLegacyNativeDescriptor(func(path string) (os.FileInfo, error) {
			require.Equal(t, "/run/voice/installation/matchmaking-native-v1.json", path)
			return nil, observation
		}), mmadmission.ErrUnavailable)
	}
	require.NoError(t, checkLegacyNativeDescriptor(func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }))
}

func TestLegacyStartupRefusesBegunAdmissionBeforeServing(t *testing.T) {
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("real PostgreSQL unavailable: set BE011_TEST_PG_DSN")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	name := "mm_legacy_start_" + uuid.NewString()[:8]
	_, err = admin.Exec(ctx, `CREATE DATABASE `+name)
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database, config.MaxConns = name, 4
	childDSN := dsn + " dbname=" + name
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		parsed.Path, parsed.RawPath = "/"+name, ""
		query := parsed.Query()
		query.Del("dbname")
		parsed.RawQuery = query.Encode()
		childDSN = parsed.String()
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(context.Background(), `DROP DATABASE `+name); admin.Close() })
	require.NoError(t, validateLegacyAdmission(ctx, pool), "actual schema-absent pre-cut compatibility")
	body, err := os.ReadFile(filepath.Join("..", "migrations", "matchmaking_db", "000024_mm_native_admission.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(body))
	require.NoError(t, err)
	require.NoError(t, validateLegacyAdmission(ctx, pool), "empty table is not a begun admission")
	_, err = pool.Exec(ctx, `INSERT INTO public.mm_native_admission_head(protocol_version,state) VALUES(1,'CLOSED')`)
	require.NoError(t, err)
	require.ErrorIs(t, validateLegacyAdmission(ctx, pool), mmadmission.ErrUnavailable)
	_, err = pool.Exec(ctx, `UPDATE public.mm_native_admission_head SET state='PREPARED'`)
	require.NoError(t, err)
	require.ErrorIs(t, validateLegacyAdmission(ctx, pool), mmadmission.ErrUnavailable)
	executable, err := os.Executable()
	require.NoError(t, err)
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	child := exec.CommandContext(bounded, executable, "-test.run=^TestLegacyStartupSubprocessMain$")
	child.Env = append(os.Environ(), "BE011_LEGACY_MAIN_CHILD=1", "DATABASE_URL="+childDSN, "MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR=invalid-bootstrap-profile")
	output, err := child.CombinedOutput()
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 1, exit.ExitCode())
	require.NoError(t, bounded.Err(), "root must reject before waiting for a listener or worker")
	require.Contains(t, string(output), "Matchmaking legacy admission unavailable")
	require.NotContains(t, string(output), "principal JWKS config")
	require.NotContains(t, string(output), childDSN)
}

// The child invokes the actual application root. Its invalid JWKS profile
// ensures a misplaced/missing local guard cannot open a test listener.
func TestLegacyStartupSubprocessMain(t *testing.T) {
	if os.Getenv("BE011_LEGACY_MAIN_CHILD") == "1" {
		main()
	}
}
