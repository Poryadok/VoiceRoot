package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

// TestProfileMutation_SearchProjectionOutboxFailureRollsBack proves a failed
// journal/outbox write cannot leave a committed profile update without its
// authoritative Search projection event.
func TestProfileMutation_SearchProjectionOutboxFailureRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))

	accountID, profileID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'projectionrollback', '0001', 'Before rollback', true)`,
		profileID, accountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		CREATE FUNCTION fail_search_projection_outbox() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'forced outbox failure'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_search_projection_outbox
		BEFORE INSERT ON user_profile_search_outbox
		FOR EACH ROW EXECUTE FUNCTION fail_search_projection_outbox();`)
	require.NoError(t, err)

	name := "Must not commit"
	updated, err := NewProfileStore(pool).UpdateOwnedProfile(
		ctx, accountID, profileID, UpdateProfileInput{DisplayName: &name})
	require.Error(t, err)
	require.Nil(t, updated)

	var displayName string
	var journalCount, outboxCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT display_name FROM profiles WHERE id = $1`, profileID).Scan(&displayName))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profile_search_journal WHERE profile_id = $1`, profileID).Scan(&journalCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profile_search_outbox`).Scan(&outboxCount))
	require.Equal(t, "Before rollback", displayName)
	require.Zero(t, journalCount)
	require.Zero(t, outboxCount)
}
