package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func startSpacePostgresForTest(t *testing.T, ctx context.Context) *SpaceMembersStore {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "spacedb", "")
	_, err := pool.Exec(ctx, `
CREATE TABLE space_members (
    space_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (space_id, profile_id)
)`)
	require.NoError(t, err)
	return &SpaceMembersStore{Pool: pool}
}

func TestSpaceMembersStore_IsAndList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	spaceStore := startSpacePostgresForTest(t, ctx)
	dm := &DMStore{Pool: integrationtest.StartPostgres(t, ctx, "chatdb2", "")}

	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	_, err := spaceStore.Pool.Exec(ctx, `
INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2), ($1, $3)
`, spaceID, profA, profB)
	require.NoError(t, err)

	row := &ChatRow{ID: uuid.New(), Type: "channel", SpaceID: &spaceID}
	ok, err := spaceStore.IsEffectiveChatMember(ctx, dm, row, profA)
	require.NoError(t, err)
	require.True(t, ok)

	members, err := spaceStore.ListEffectiveChatMembers(ctx, dm, row)
	require.NoError(t, err)
	require.Len(t, members, 2)
}
