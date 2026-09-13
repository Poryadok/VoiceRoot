package grpcsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	userv1 "voice.app/voice/user/v1"
)

type settingsEventsRecorder struct {
	profileUpdated []string
	settingsKeys   []string
	settingsJSON   string
}

func (r *settingsEventsRecorder) PublishProfileCreated(context.Context, string, string) error {
	return nil
}
func (r *settingsEventsRecorder) PublishProfileUpdated(_ context.Context, _ string, changedFields []string) error {
	r.profileUpdated = changedFields
	return nil
}
func (r *settingsEventsRecorder) PublishProfileSwitched(context.Context, string, string, string) error {
	return nil
}
func (r *settingsEventsRecorder) PublishVerified(context.Context, string, string) error { return nil }
func (r *settingsEventsRecorder) PublishPresenceChanged(context.Context, string, string, string) error {
	return nil
}
func (r *settingsEventsRecorder) PublishGameDetected(context.Context, string, string) error {
	return nil
}
func (r *settingsEventsRecorder) PublishSettingsChanged(_ context.Context, _ string, changedKeys []string, changedKeysJSON string) error {
	r.settingsKeys = changedKeys
	r.settingsJSON = changedKeysJSON
	return nil
}

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

func TestPublishSettingsChanges_PreservesLegacyProfileUpdate(t *testing.T) {
	t.Parallel()
	recorder := &settingsEventsRecorder{}
	publishSettingsChanges(context.Background(), recorder, "profile-1", []string{"language", "theme"})
	require.Equal(t, []string{"settings"}, recorder.profileUpdated)
	require.Equal(t, []string{"language", "theme"}, recorder.settingsKeys)
	require.Equal(t, `["language","theme"]`, recorder.settingsJSON)
}
