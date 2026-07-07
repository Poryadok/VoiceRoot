package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

// TestSearchUsers_VerifiedProfilesRankedHigher documents verified profiles sort above unverified matches.
func TestSearchUsers_VerifiedProfilesRankedHigher_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)

	phase13Path := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000002_phase13_verification_type.up.sql")
	phase13SQL, err := os.ReadFile(phase13Path)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(phase13SQL))
	require.NoError(t, err)

	viewer := uuid.New()
	unverifiedID := uuid.New()
	verifiedID := uuid.New()

	st := NewProfileSpaceSearchStore(pool)
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     unverifiedID,
		AccountID:     uuid.New(),
		Username:      "aaafaker",
		Discriminator: "0001",
		DisplayName:   "Faker Copy",
	}))
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     verifiedID,
		AccountID:     uuid.New(),
		Username:      "zzzfaker",
		Discriminator: "0002",
		DisplayName:   "Faker Official",
	}))
	_, err = pool.Exec(ctx, `
		UPDATE profile_search_documents SET verification_type = 'personal' WHERE profile_id = $1`, verifiedID)
	require.NoError(t, err)

	hits, err := st.SearchProfiles(ctx, viewer, "faker", nil, 20)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(hits), 2)
	require.Equal(t, verifiedID, hits[0].ProfileID,
		"SearchUsers must rank verified profiles above unverified for the same query")
}
