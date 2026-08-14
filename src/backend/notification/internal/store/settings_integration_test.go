package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/store"
)

func TestSettingsStore_UpsertAndGet_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	s := &store.SettingsStore{Pool: pool}
	profileID := uuid.New()
	muteUntil := time.Now().UTC().Add(time.Hour)

	err := s.UpsertSettings(ctx, store.NotificationSettings{
		ProfileID:     profileID,
		ScopeType:     "global",
		Enabled:       false,
		MuteUntil:     &muteUntil,
		SuppressTypes: []string{"new_message", "system"},
	})
	require.NoError(t, err)

	got, err := s.GetSettings(ctx, profileID, "global", nil)
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.Equal(t, []string{"new_message", "system"}, got.SuppressTypes)
	require.NotNil(t, got.MuteUntil)
}

func TestSettingsStore_QuietHours_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	s := &store.SettingsStore{Pool: pool}
	profileID := uuid.New()

	err := s.SetQuietHours(ctx, store.QuietHours{
		ProfileID:        profileID,
		Enabled:          true,
		StartTime:        "23:00",
		EndTime:          "08:00",
		Timezone:         "UTC",
		OverrideMentions: true,
	})
	require.NoError(t, err)

	got, err := s.GetQuietHours(ctx, profileID)
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, "23:00", got.StartTime)
	require.Equal(t, "08:00", got.EndTime)
}
