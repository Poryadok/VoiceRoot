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

func TestNativeReadinessActualRemoveFinalizesFullCohort(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	space, owner, target, survivor := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := db.Pool.Exec(ctx, `INSERT INTO public.spaces(id,name,owner_profile_id,member_count) VALUES($1,'actual guarded removal',$2,3)`, space, owner)
	require.NoError(t, err)
	for _, profile := range []uuid.UUID{owner, target, survivor} {
		_, err = db.Pool.Exec(ctx, `INSERT INTO public.space_members(space_id,profile_id) VALUES($1,$2)`, space, profile)
		require.NoError(t, err)
	}
	db.Run(true, "up")
	require.NoError(t, nativeReadinessTx(ctx, db.Pool, space, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: db.Pool, tx: tx}).InitializeScopedNativeMembership(ctx, space)
	}))
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	st := &SpaceStore{Pool: serving}
	require.NoError(t, st.RemoveMember(ctx, space, target), "actual domain writer must capture and complete before commit")
	var state, boundary string
	var generation, floor, count int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state,generation,retired_through FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space, target).Scan(&state, &generation, &floor))
	require.Equal(t, "RETIRED", state)
	require.Equal(t, generation, floor)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state,boundary_state,generation FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space, survivor).Scan(&state, &boundary, &generation))
	require.Equal(t, "ACTIVE", state)
	require.Equal(t, "UNKNOWN", boundary)
	require.EqualValues(t, 1, generation, "unchanged cohort member keeps its real initialized history")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM public.spaces WHERE id=$1`, space).Scan(&count))
	require.EqualValues(t, 2, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count, "actual commit must dispose the whole private capture")
	require.NoError(t, nativeReadinessTx(ctx, serving, space, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: serving, tx: tx}).RequireScopedNativeMembershipReadiness(ctx, space)
	}))
}
