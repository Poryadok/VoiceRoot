package limits_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/subscription/internal/limits"
)

func decode(t *testing.T, raw string) map[string]int64 {
	t.Helper()
	var values map[string]int64
	require.NoError(t, json.Unmarshal([]byte(raw), &values))
	return values
}

// Expectations are literal product values, independent of production fixtures.
func TestPersonalBenefitMatrix(t *testing.T) {
	flags := []string{"premium_badge", "animated_avatar", "profile_banner", "custom_status", "emoji_status", "primary_premium_username", "exclusive_stickers", "exclusive_emoji", "custom_app_icons", "exclusive_chat_themes", "file_download_priority"}
	for _, tc := range []struct {
		tier string
		paid bool
	}{{"free", false}, {"premium", true}, {"grace_period", true}, {"", false}, {"space_pro", false}, {"PREMIUM", false}, {"cancelled", false}} {
		t.Run(tc.tier, func(t *testing.T) {
			want := map[string]int64{"file_upload_bytes": 52_428_800, "profile_count": 2, "file_retention_days": 90, "e2e_file_retention_days": 90, "message_reaction_count": 1}
			var enabled int64
			if tc.paid {
				enabled = 1
				want["file_upload_bytes"] = 209_715_200
				want["profile_count"] = 5
				want["file_retention_days"] = 0 // Unlimited non-E2E retention.
				want["message_reaction_count"] = 3
			}
			for _, key := range flags {
				want[key] = enabled
			}
			require.Equal(t, want, decode(t, limits.ForAccount(tc.tier)))
		})
	}
}

func TestSpaceBenefitMatrix(t *testing.T) {
	for _, pro := range []bool{false, true} {
		want := map[string]int64{"space_member_count": 50, "voice_room_participants": 32, "space_tree_nodes": 50, "space_banner": 0, "space_custom_emoji": 0}
		if pro {
			want = map[string]int64{"space_member_count": 5000, "voice_room_participants": 128, "space_tree_nodes": 500, "space_banner": 1, "space_custom_emoji": 1}
		}
		got := decode(t, limits.ForSpace(pro))
		require.Equal(t, want, got)
		require.Equal(t, got["space_member_count"], limits.SpaceMemberCap(pro))
		require.Equal(t, got["voice_room_participants"], int64(limits.VoiceRoomCap(pro)))
	}
}
