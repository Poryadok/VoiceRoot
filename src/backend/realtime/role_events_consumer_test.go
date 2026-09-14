package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoleEventToFanout_ChatOverrideRemovedTargetsChatSubscribers(t *testing.T) {
	payload, err := json.Marshal(roleEventJSON{
		SpaceID: "11111111-1111-1111-1111-111111111111",
		ChatID:  "22222222-2222-2222-2222-222222222222",
		RoleID:  "33333333-3333-3333-3333-333333333333",
	})
	require.NoError(t, err)

	profileID, chatID, envelope, ok := roleEventToFanout("role.chat_override_removed", payload)
	require.True(t, ok)
	require.Empty(t, profileID)
	require.Equal(t, "22222222-2222-2222-2222-222222222222", chatID)
	require.Equal(t, "role_update", envelope.Op)
	require.JSONEq(t, `{
		"space_id":"11111111-1111-1111-1111-111111111111",
		"role_id":"33333333-3333-3333-3333-333333333333",
		"profile_id":"",
		"name":"",
		"chat_id":"22222222-2222-2222-2222-222222222222",
		"voice_room_id":"",
		"subject":"role.chat_override_removed"
	}`, string(envelope.D))
}
