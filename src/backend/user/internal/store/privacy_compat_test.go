package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/pkg/privacy"
)

func TestShowLastSeenAudienceFromColumn_ExpandCompatibility(t *testing.T) {
	explicit := privacy.Audience{Friends: true, SpaceMembers: true}
	encoded, err := privacy.MarshalJSON(explicit)
	require.NoError(t, err)

	cases := []struct {
		name   string
		preset string
		raw    []byte
		want   privacy.Audience
	}{
		{name: "personal legacy null", preset: "personal", want: privacy.FriendsOnly()},
		{name: "work legacy null", preset: "work", want: privacy.SpaceMembersOnly()},
		{name: "gaming legacy null", preset: "gaming", want: privacy.EveryoneWithGuests()},
		{name: "explicit value is preserved", preset: "personal", raw: encoded, want: explicit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := showLastSeenAudienceFromColumn(tc.raw, tc.preset)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMigration000013_ShowLastSeenRemainsExpandCompatible(t *testing.T) {
	root := userStoreRepoRoot(t)
	up, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "user_db", "000013_privacy_show_last_seen.up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "user_db", "000013_privacy_show_last_seen.down.sql"))
	require.NoError(t, err)

	normalizedUp := strings.ToLower(string(up))
	require.Contains(t, normalizedUp, "add column if not exists show_last_seen_audience jsonb")
	require.NotContains(t, normalizedUp, "alter column show_last_seen_audience set not null", "old writers omit this additive column during expand")
	require.NotContains(t, normalizedUp, "show_last_seen_audience default", "preset-specific defaults belong to the reader, not a universal SQL default")
	require.Contains(t, strings.ToLower(string(down)), "drop column if exists show_last_seen_audience")
}
