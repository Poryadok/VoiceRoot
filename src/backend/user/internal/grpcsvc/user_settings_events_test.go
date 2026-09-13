package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"

	userv1 "voice.app/voice/user/v1"
)

func TestSettingsChangedKeys_OnlyReportsSuppliedMutations(t *testing.T) {
	t.Parallel()

	require.Nil(t, settingsChangedKeys(nil))
	require.Empty(t, settingsChangedKeys(&userv1.UserSettings{}))
	require.Equal(t, []string{"language", "theme", "notification_prefs_json"}, settingsChangedKeys(&userv1.UserSettings{
		Language:              "en",
		Theme:                 "dark",
		NotificationPrefsJson: `{"dm":true}`,
	}))
}
