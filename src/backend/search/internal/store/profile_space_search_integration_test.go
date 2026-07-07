package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func TestProfileSpaceSearchStore_ProfileILIKE_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000002_verification_type.up.sql"))

	viewer := uuid.New()
	pidCarol := uuid.New()
	pidDave := uuid.New()

	st := NewProfileSpaceSearchStore(pool)
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     pidCarol,
		AccountID:     uuid.New(),
		Username:      "carol",
		Discriminator: "0001",
		DisplayName:   "Carol Literal 50%_off",
	}))
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     pidDave,
		AccountID:     uuid.New(),
		Username:      "dave",
		Discriminator: "0002",
		DisplayName:   "Dave",
	}))

	t.Run("username ilike match", func(t *testing.T) {
		hits, err := st.SearchProfiles(ctx, viewer, "carol", nil, 20)
		require.NoError(t, err)
		require.Len(t, hits, 1)
		require.Equal(t, pidCarol, hits[0].ProfileID)
	})

	t.Run("literal percent and underscore in display_name", func(t *testing.T) {
		hits, err := st.SearchProfiles(ctx, viewer, "50%_off", nil, 20)
		require.NoError(t, err)
		require.Len(t, hits, 1)
		require.Equal(t, pidCarol, hits[0].ProfileID)
	})
}

func TestProfileSpaceSearchStore_ExcludesBlockedProfiles_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000002_verification_type.up.sql"))

	viewer := uuid.New()
	blockedAccount := uuid.New()
	allowedProfile := uuid.New()
	allowedAccount := uuid.New()

	st := NewProfileSpaceSearchStore(pool)
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     uuid.New(),
		AccountID:     blockedAccount,
		Username:      "blockeduser",
		Discriminator: "0001",
		DisplayName:   "Blocked User",
	}))
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     allowedProfile,
		AccountID:     allowedAccount,
		Username:      "alloweduser",
		Discriminator: "0002",
		DisplayName:   "Allowed User",
	}))

	hits, err := st.SearchProfiles(ctx, viewer, "user", []uuid.UUID{blockedAccount}, 20)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, allowedProfile, hits[0].ProfileID)
}

func TestProfileSpaceSearchStore_SpaceVisibilityFilter_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)

	publicID := uuid.New()
	inviteID := uuid.New()
	privateID := uuid.New()

	st := NewProfileSpaceSearchStore(pool)
	require.NoError(t, st.UpsertSpace(ctx, SpaceDocument{
		SpaceID:    publicID,
		Name:       "Public Raiders",
		Visibility: "public",
	}))
	require.NoError(t, st.UpsertSpace(ctx, SpaceDocument{
		SpaceID:    inviteID,
		Name:       "Invite Raiders",
		Visibility: "invite_only",
	}))
	require.NoError(t, st.UpsertSpace(ctx, SpaceDocument{
		SpaceID:    privateID,
		Name:       "Private Raiders",
		Visibility: "private",
	}))

	hits, _, err := st.SearchSpaces(ctx, "Raiders", nil, 20)
	require.NoError(t, err)
	require.Len(t, hits, 2)
	ids := map[uuid.UUID]bool{hits[0].SpaceID: true, hits[1].SpaceID: true}
	require.True(t, ids[publicID])
	require.True(t, ids[inviteID])
	require.False(t, ids[privateID])
}
