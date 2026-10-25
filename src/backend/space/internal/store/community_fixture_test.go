package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// legacyCommunityFixture seeds an existing community before the native cutover.
// The real migration adopts ordinary memberships; tests never forge native heads.
func legacyCommunityFixture(t *testing.T, ctx context.Context, in CommunityBootstrapSpaceInput,
	seed func(*SpaceStore, *SpaceRow)) (*SpaceStore, *SpaceRow) {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "37")
	st := &SpaceStore{Pool: db.Pool}
	space, err := st.CreateSpace(ctx, in.OwnerProfileID, "Existing community", "", "private")
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO community_owner_authority
		(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status)
		VALUES($1,$2,$3,$4,$5,$6,1,'active')`, space.ID, in.ApplicationID, in.EnvironmentID,
		in.CorporationKey, in.OwnerAccountID, in.OwnerProfileID)
	require.NoError(t, err)
	if seed != nil {
		seed(st, space)
	}
	db.Run(true, "goto", "39")
	return st, space
}
