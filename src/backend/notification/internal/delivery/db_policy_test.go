package delivery_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
)

type stubSettingsReader struct {
	global delivery.SettingsRecord
	chat   delivery.SettingsRecord
	quiet  delivery.QuietHoursRecord
}

func (s stubSettingsReader) GetSettings(_ context.Context, _ uuid.UUID, scopeType string, scopeID *uuid.UUID) (delivery.SettingsRecord, error) {
	if scopeType == "chat" && scopeID != nil {
		return s.chat, nil
	}
	return s.global, nil
}

func (s stubSettingsReader) GetQuietHours(context.Context, uuid.UUID) (delivery.QuietHoursRecord, error) {
	return s.quiet, nil
}

func TestDBPolicyLoader_AppliesChatMuteAndQuietHours(t *testing.T) {
	chatID := uuid.New()
	loader := delivery.DBPolicyLoader{Reader: stubSettingsReader{
		global: delivery.SettingsRecord{Enabled: true},
		chat:   delivery.SettingsRecord{Enabled: false},
		quiet: delivery.QuietHoursRecord{
			Enabled:   true,
			StartTime: "23:00",
			EndTime:   "08:00",
			Timezone:  "UTC",
		},
	}}
	settings, quiet, err := loader.LoadPolicy(context.Background(), uuid.New(), chatID.String(), delivery.TypeNewMessage, time.Date(2026, 6, 12, 23, 30, 0, 0, time.UTC))
	require.NoError(t, err)
	require.True(t, settings.ChatMuted)
	require.True(t, quiet.Enabled)
}
