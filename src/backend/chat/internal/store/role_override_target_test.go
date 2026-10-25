package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
)

func TestRoleOverrideTargetUsesOwnChatAndLifecycleState(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	st := &DMStore{Pool: pool}
	space, foreign, creator := uuid.New(), uuid.New(), uuid.New()
	create := func(kind string, spaceID *uuid.UUID) uuid.UUID {
		id := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO chats(id,type,space_id,name,creator_profile_id) VALUES($1,$2,$3,'target',$4)`, id, kind, spaceID, creator)
		require.NoError(t, err)
		return id
	}
	group, channel := create("group", &space), create("channel", &space)
	require.NoError(t, st.ValidateRoleOverrideTarget(ctx, space, group))
	require.NoError(t, st.ValidateRoleOverrideTarget(ctx, space, channel))
	for _, id := range []uuid.UUID{uuid.New(), create("group", nil), create("dm", nil), create("channel", &foreign)} {
		require.ErrorIs(t, st.ValidateRoleOverrideTarget(ctx, space, id), pgx.ErrNoRows)
	}
	_, err := pool.Exec(ctx, `UPDATE chats SET type='channel' WHERE id=$1`, group)
	require.NoError(t, err)
	require.NoError(t, st.ValidateRoleOverrideTarget(ctx, space, group))
	_, err = pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, group)
	require.NoError(t, err)
	require.ErrorIs(t, st.ValidateRoleOverrideTarget(ctx, space, group), pgx.ErrNoRows)
	_, err = (&SpaceLifecycleStore{Pool: pool}).PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: uuid.NewString(), ScheduleGeneration: 1})
	require.NoError(t, err)
	require.ErrorIs(t, st.ValidateRoleOverrideTarget(ctx, space, channel), ErrSpaceLifecycleState)
}
