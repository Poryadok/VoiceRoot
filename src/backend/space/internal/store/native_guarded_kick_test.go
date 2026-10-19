package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestNative44KickCommitsHistoryAuditAndFullCohort(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	owner, target, survivor := uuid.New(), uuid.New(), uuid.New()
	st := &SpaceStore{Pool: db.Pool}
	space, err := st.CreateSpace(ctx, owner, "Guarded administrative kick", "", "public")
	require.NoError(t, err)
	for _, profile := range []uuid.UUID{target, survivor} {
		_, err = db.Pool.Exec(ctx, `INSERT INTO public.space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, profile)
		require.NoError(t, err)
	}
	_, err = db.Pool.Exec(ctx, `UPDATE public.spaces SET member_count=3 WHERE id=$1`, space.ID)
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, st.InitializeNativeMembership(ctx, space.ID))
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	require.NoError(t, serving.KickMember(ctx, space.ID, target, owner))
	var state, boundary string
	var generation, floor, count int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state,generation,retired_through FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, target).Scan(&state, &generation, &floor))
	require.Equal(t, "RETIRED", state)
	require.Equal(t, generation, floor)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state,boundary_state FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, survivor).Scan(&state, &boundary))
	require.Equal(t, "ACTIVE", state)
	require.Equal(t, "UNKNOWN", boundary)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.audit_log WHERE space_id=$1 AND actor_profile_id=$2 AND target_id=$3 AND action='member_kicked'`, space.ID, owner, target).Scan(&count))
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM public.spaces WHERE id=$1`, space.ID).Scan(&count))
	require.EqualValues(t, 2, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count)
}
