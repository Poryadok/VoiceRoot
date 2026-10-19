package nativestartup

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestNativeDatabaseNeverReturnsOptionalSuccess(t *testing.T) {
	for _, connection := range []string{"", " ", "postgres://invalid:notlogged@localhost:bad/database"} {
		pool, err := OpenDatabase(context.Background(), completeDescriptor(), connection)
		require.ErrorIs(t, err, ErrInvalidInstallation)
		require.Nil(t, pool)
		require.NotContains(t, err.Error(), "notlogged")
	}
	pool, err := OpenDatabase(context.Background(), Descriptor{}, "postgres://unused")
	require.ErrorIs(t, err, ErrInvalidInstallation)
	require.Nil(t, pool, "incomplete descriptor must be rejected before dialing")
}

func TestNativeDatabaseRejectsActualWrongRoleAndMissingCoverage(t *testing.T) {
	connection := os.Getenv("BE011_TEST_PG_DSN")
	if connection == "" {
		t.Skip("requires owned Linux PostgreSQL fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, connection)
	require.NoError(t, err)
	defer pool.Close()
	descriptor := completeDescriptor()
	require.ErrorIs(t, CheckDatabase(ctx, pool, descriptor), ErrInvalidInstallation, "a configured role name is not authenticated login evidence")
	require.NoError(t, pool.QueryRow(ctx, `SELECT current_user::text,(SELECT oid FROM pg_roles WHERE rolname=current_user)`).Scan(&descriptor.ExpectedDatabaseLoginRoleName, &descriptor.ExpectedDatabaseLoginRoleOID))
	require.ErrorIs(t, CheckDatabase(ctx, pool, descriptor), ErrInvalidInstallation, "matching actual role cannot replace restricted privileges and ACTIVE coverage")
	require.Zero(t, pool.Stat().AcquiredConns())
}
