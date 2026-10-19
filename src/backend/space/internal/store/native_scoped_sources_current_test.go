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

func TestNativeScopedSourcesCurrent41FenceAndSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native cohort")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Scoped source snapshot", "", "public")
	require.NoError(t, err)
	member := uuid.New()
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, member)
	require.NoError(t, err)
	db.Run(true, "up")
	_, err = st.ReadScopedMemberSourceBindings(ctx, space.ID, owner)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
	old, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(ctx)
	var count int
	require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, st.RemoveMember(ctx, space.ID, member))
	_, err = old.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(space.ID))
	require.NoError(t, err)
	stale := *st
	stale.tx = old
	_, err = stale.ReadScopedMemberSourceBindings(ctx, space.ID, member)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "actual fence cannot refresh an older RR snapshot")
	require.NoError(t, old.Rollback(ctx))
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		bindings, err := scoped.ReadScopedMemberSourceBindings(ctx, space.ID, owner)
		require.NoError(t, err)
		require.Len(t, bindings, 1)
		require.True(t, bindings[0].GetOrdinary().GetLegacy())
		_, err = scoped.ReadScopedMemberSourceBindings(ctx, space.ID, member)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		return nil
	}))
}
