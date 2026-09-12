package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/privacy"
)

func TestPrivacyStore_ExpandMigrationAcceptsLegacyWriterAndReadsPresetFallback(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userStoreRepoRoot(t))

	profileID := uuid.New()
	const audience = `{"friends":true}`
	_, err := pool.Exec(ctx, `
INSERT INTO privacy_settings (
  profile_id, preset,
  show_online_audience, show_game_status_audience, show_mm_rating_audience, show_phone_audience, show_stories_audience,
  allow_phone_search_audience, allow_dm_audience, allow_calls_audience, allow_chat_space_invites_audience,
  allow_files_audience, allow_voice_messages_audience, allow_friend_requests_audience,
  allow_guest_dm, allow_forward, show_read_receipts
) VALUES (
  $1, 'work',
  $2::jsonb, $2::jsonb, $2::jsonb, $2::jsonb, $2::jsonb,
  $2::jsonb, $2::jsonb, $2::jsonb, $2::jsonb,
  $2::jsonb, $2::jsonb, $2::jsonb,
  false, true, true
)`, profileID, audience)
	require.NoError(t, err, "the pre-000013 writer form must survive the expand migration")

	row, err := NewPrivacyStore(pool).GetByProfileID(ctx, profileID)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Equal(t, privacy.SpaceMembersOnly(), row.ShowLastSeen,
		"NULL additive field uses the documented work-preset default until every writer upgrades")
}
