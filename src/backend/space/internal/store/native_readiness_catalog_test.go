package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// The real installer establishes UNKNOWN once; the real application LOGIN
// must subsequently qualify the exact installed44 contract without becoming
// an installer or treating raw head presence as a serving grant.
func TestNativeReadinessQualified44ServingCatalog(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	space, profile := uuid.New(), uuid.New()
	_, err := db.Pool.Exec(ctx, `INSERT INTO public.spaces(id,name,owner_profile_id) VALUES($1,'qualified guard44',$2)`, space, profile)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO public.space_members(space_id,profile_id) VALUES($1,$2)`, space, profile)
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	require.NoError(t, nativeReadinessTx(ctx, db.Pool, space, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `SELECT public.space_native_membership_initialize($1)`, space)
		return e
	}))
	require.NoError(t, nativeReadinessTx(ctx, serving, space, func(tx pgx.Tx) error {
		require.NoError(t, checkMemberAuthorityCatalog(ctx, tx), "native source/guard closure qualification")
		return (&SpaceStore{Pool: serving, tx: tx}).RequireScopedNativeMembershipReadiness(ctx, space)
	}), "exact44 + actual nonowner login + completed full cutover must qualify")
	require.Error(t, nativeReadinessTx(ctx, db.Pool, space, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: db.Pool, tx: tx}).RequireScopedNativeMembershipReadiness(ctx, space)
	}), "an elevated installer session is never a serving positive")
	var shape string
	require.NoError(t, db.Pool.QueryRow(ctx, nativeReadinessStructureSQL).Scan(&shape))
	t.Log("GUARD_SHAPE", shape)
	t.Run("unrecognized private shape cannot be self sealed", func(t *testing.T) {
		tx, e := db.Pool.Begin(ctx)
		require.NoError(t, e)
		defer tx.Rollback(ctx)
		_, e = tx.Exec(ctx, `ALTER TABLE native_membership_guard.frames ADD COLUMN unqualified_authority boolean;
 UPDATE native_membership_guard.epoch SET catalog_sha256=native_membership_guard.catalog()`)
		require.NoError(t, e)
		require.Error(t, checkNativeReadinessCatalog(ctx, tx), "an installer-selected hash is not a fixed native structural qualification")
	})
	for name, sql := range map[string]string{
		"disabled source guard":          `ALTER TABLE public.space_members DISABLE TRIGGER a000_native_membership_capture`,
		"disabled deferred commit guard": `ALTER TABLE native_membership_guard.frames DISABLE TRIGGER membership_frame_completed`,
		"mutable private registry":       `GRANT UPDATE ON native_membership_guard.spaces TO PUBLIC`,
		"public installer":               `GRANT EXECUTE ON FUNCTION public.space_native_membership_initialize(uuid) TO PUBLIC`,
		"function search path":           `ALTER FUNCTION public.space_native_membership_begin(uuid,uuid[]) SET search_path=public,pg_catalog`,
		"completion default":             `CREATE OR REPLACE FUNCTION native_membership_guard.verify_completion(s uuid,seal boolean DEFAULT false) RETURNS void LANGUAGE plpgsql AS 'BEGIN RETURN; END'`,
		"future schema":                  `UPDATE public.schema_migrations SET version=45`,
		"dirty schema":                   `UPDATE public.schema_migrations SET dirty=true`,
	} {
		t.Run(name, func(t *testing.T) {
			tx, e := db.Pool.Begin(ctx)
			require.NoError(t, e)
			defer tx.Rollback(ctx)
			_, e = tx.Exec(ctx, sql)
			require.NoError(t, e)
			require.Error(t, checkMemberAuthorityCatalog(ctx, tx))
		})
	}
}
