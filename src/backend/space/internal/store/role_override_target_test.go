package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRoleOverrideTargetUsesOwnSpaceAndRoomState(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "target", "", "private")
	require.NoError(t, err)
	foreign, err := st.CreateSpace(ctx, uuid.New(), "foreign", "", "private")
	require.NoError(t, err)
	room, _, err := st.CreateVoiceRoom(ctx, space.ID, "voice", nil)
	require.NoError(t, err)
	other, _, err := st.CreateVoiceRoom(ctx, foreign.ID, "other", nil)
	require.NoError(t, err)
	require.NoError(t, st.ValidateRoleOverrideTarget(ctx, space.ID, nil))
	require.NoError(t, st.ValidateRoleOverrideTarget(ctx, space.ID, &room.ID))
	require.ErrorIs(t, st.ValidateRoleOverrideTarget(ctx, space.ID, &other.ID), pgx.ErrNoRows)
	require.ErrorIs(t, st.ValidateRoleOverrideTarget(ctx, uuid.New(), nil), pgx.ErrNoRows)
	missing := uuid.New()
	require.ErrorIs(t, st.ValidateRoleOverrideTarget(ctx, space.ID, &missing), pgx.ErrNoRows)
	require.NoError(t, st.DeleteVoiceRoom(ctx, room.ID))
	require.ErrorIs(t, st.ValidateRoleOverrideTarget(ctx, space.ID, &room.ID), pgx.ErrNoRows)
}
