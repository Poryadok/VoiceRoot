package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func TestAreCoMembers_SharedSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "spacedb", "")
	applySpaceMigrationForStoreTest(t, ctx, pool)
	store := &SpaceStore{Pool: pool}

	owner := uuid.New()
	memberA := uuid.New()
	memberB := uuid.New()
	spaceID := uuid.New()

	_, err := pool.Exec(ctx, `
INSERT INTO spaces (id, name, description, visibility, owner_profile_id, member_count)
VALUES ($1, 'team', '', 'private', $2, 2)`,
		spaceID, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2), ($1, $3)`,
		spaceID, memberA, memberB)
	require.NoError(t, err)

	ok, err := store.AreCoMembers(ctx, memberA, memberB, nil)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = store.AreCoMembers(ctx, memberA, uuid.New(), nil)
	require.NoError(t, err)
	require.False(t, ok)
}
