package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeCatalogServerAndSchemaFamiliesAreClosed(t *testing.T) {
	for _, server := range []int{0, 150000, 170000, 190000} {
		_, err := nativeReadinessExpectedStructure(45, server)
		require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
	}
	for _, server := range []int{160000, 180006} {
		for _, schema := range []int64{43, 46} {
			_, err := nativeReadinessExpectedStructure(schema, server)
			require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
		}
	}
}

func TestNativePostgres18CatalogTransitionAndBornPublication(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	var server int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer`).Scan(&server))
	require.Equal(t, 18, server/10000)
	check := func(version int) error {
		var structure string
		require.NoError(t, db.Pool.QueryRow(ctx, nativeReadinessStructureSQL).Scan(&structure))
		t.Logf("PostgreSQL18 schema%d structure SHA256=%x", version, sha256.Sum256([]byte(structure)))
		require.NoError(t, os.WriteFile(fmt.Sprintf("/tmp/native-pg18-%d-structure.json", version), []byte(structure), 0600))
		tx, err := db.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		return checkMemberAuthorityCatalog(ctx, tx)
	}
	db.Run(true, "goto", "44")
	err44 := check(44)
	db.Run(true, "goto", "45")
	err45 := check(45)
	require.NoError(t, err44, "canonical PostgreSQL18 schema44 must qualify without adopting a runtime hash")
	require.NoError(t, err45, "canonical PostgreSQL18 exact M45 metadata must qualify")
	st := &SpaceStore{Pool: db.Pool}
	receipt, err := st.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.True(t, receipt.Changed)
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	create, _, _ := signedNativeCreatorCurrent(t, serving, &spacev1.CreateSpaceRequest{Name: "PostgreSQL18 born", Visibility: "private"})
	_, err = create()
	require.NoError(t, err, "genuine serving LOGIN publishes Creator through the native DB frame")
	for _, mutation := range []struct{ name, sql string }{
		{"not_null_removed", `ALTER TABLE native_membership_guard.frames ALTER COLUMN mode DROP NOT NULL`},
		{"not_null_renamed", `ALTER TABLE native_membership_guard.frames RENAME CONSTRAINT frames_mode_not_null TO unqualified_mode_not_null`},
		{"guard_disabled", `ALTER TABLE public.space_members DISABLE TRIGGER a000_native_membership_capture`},
		{"future_schema", `UPDATE public.schema_migrations SET version=46`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			_, err = tx.Exec(ctx, mutation.sql)
			require.NoError(t, err)
			require.ErrorIs(t, checkMemberAuthorityCatalog(ctx, tx), ErrMemberAuthorityUnavailable)
		})
	}
}
