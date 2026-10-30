package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

// Exercise the deployed migrator and public reader together: a version number
// alone is not evidence that its associated catalog is present.
func TestUserAuthorityReaderActualDriver21To22(t *testing.T) {
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(userModuleRepoRoot(t), "src", "backend", "migrations", "user_db"))
	f.Run(true, "goto", "19")
	st := NewProfileStore(f.Pool)
	profile, account := uuid.NewString(), uuid.NewString()
	_, err := f.Pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'additive22','0001','private')`, profile, account)
	require.NoError(t, err)
	f.Run(true, "goto", "20")
	f.Run(true, "goto", "21")
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), ProfileIds: []string{profile}}
	require.NoError(t, st.CheckAuthoritySourceSchema(ctx))
	before, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.True(t, before.Complete)
	state, err := authoritysource.DecodeUserState(before.CanonicalState)
	require.NoError(t, err)
	require.True(t, state.EligibleProfile(profile, account))
	_, err = f.Pool.Exec(ctx, `UPDATE schema_migrations SET version=22`)
	require.NoError(t, err)
	require.Error(t, st.CheckAuthoritySourceSchema(ctx), "a marker alone does not install M22")
	_, err = st.ReadAuthorityRevision(ctx, scope)
	require.Error(t, err)
	_, err = st.ReadAuthoritySnapshot(ctx, scope)
	require.Error(t, err)
	_, err = f.Pool.Exec(ctx, `UPDATE schema_migrations SET version=21`)
	require.NoError(t, err)

	f.Run(true, "goto", "22")
	var version int64
	var dirty bool
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 22, version)
	require.False(t, dirty)
	require.NoError(t, st.CheckAuthoritySourceSchema(ctx))
	after, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	require.True(t, after.Complete)
	require.Equal(t, before.Revision, after.Revision)
	require.Equal(t, before.CanonicalState, after.CanonicalState)
	revision, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, after.Revision, revision)

	// Catalog locks must cover M22 relations for the lifetime of the same
	// transaction that supplies the owner cut. A DDL writer cannot slip between
	// qualification and its read.
	tx, err := st.beginAuthorityRead(ctx, scope)
	require.NoError(t, err)
	ddl, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = ddl.Exec(ctx, `SET LOCAL lock_timeout='200ms'`)
	require.NoError(t, err)
	_, err = ddl.Exec(ctx, `ALTER TABLE identity_meta ENABLE ROW LEVEL SECURITY`)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr, "DDL must wait for the qualified read cut")
	require.Equal(t, "55P03", pgErr.Code)
	require.NoError(t, ddl.Rollback(ctx))
	require.NoError(t, tx.Commit(ctx))

	// Both preflight and reads must fail closed on unknown or dirty state.
	for _, test := range []struct{ name, change, restore string }{
		{"dirty", `UPDATE schema_migrations SET dirty=true`, `UPDATE schema_migrations SET dirty=false`},
		{"unknown", `UPDATE schema_migrations SET version=999`, `UPDATE schema_migrations SET version=22`},
		{"false downgrade", `UPDATE schema_migrations SET version=21`, `UPDATE schema_migrations SET version=22`},
		{"missing identity relation", `ALTER TABLE identity_meta RENAME TO identity_meta_missing`, `ALTER TABLE identity_meta_missing RENAME TO identity_meta`},
		{"identity relation", `ALTER TABLE identity_meta ENABLE ROW LEVEL SECURITY`, `ALTER TABLE identity_meta DISABLE ROW LEVEL SECURITY`},
		{"identity guard", `ALTER TABLE profiles DISABLE TRIGGER identity_profile_registrar`, `ALTER TABLE profiles ENABLE TRIGGER identity_profile_registrar`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.Pool.Exec(ctx, test.change)
			require.NoError(t, err)
			defer func() { _, restoreErr := f.Pool.Exec(ctx, test.restore); require.NoError(t, restoreErr) }()
			require.Error(t, st.CheckAuthoritySourceSchema(ctx))
			_, err = st.ReadAuthorityRevision(ctx, scope)
			require.Error(t, err)
			_, err = st.ReadAuthoritySnapshot(ctx, scope)
			require.Error(t, err)
		})
	}
	var original string
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT pg_get_functiondef('identity_profile_registrar_guard()'::regprocedure)`).Scan(&original))
	_, err = f.Pool.Exec(ctx, `CREATE OR REPLACE FUNCTION identity_profile_registrar_guard() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$`)
	require.NoError(t, err)
	require.Error(t, st.CheckAuthoritySourceSchema(ctx))
	_, err = st.ReadAuthorityRevision(ctx, scope)
	require.Error(t, err)
	_, err = f.Pool.Exec(ctx, original)
	require.NoError(t, err)
	require.NoError(t, st.CheckAuthoritySourceSchema(ctx))
}
