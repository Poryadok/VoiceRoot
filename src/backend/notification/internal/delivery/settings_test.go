package delivery_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
)

func baseOnlineDecision() delivery.DeliveryDecision {
	return delivery.DeliveryDecision{InApp: true, Push: true}
}

func TestApplySettings_MutedChatSuppressesNewMessage(t *testing.T) {
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		SenderProfileID:    uuid.New(),
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	}
	settings := delivery.SettingsSnapshot{
		ChatMuted:            true,
		MentionOverridesMute: true,
	}
	out := delivery.ApplySettings(baseOnlineDecision(), in, settings)
	require.False(t, out.Push, "muted chat suppresses new_message push")
	require.False(t, out.InApp, "muted chat suppresses new_message in-app")
}

func TestApplySettings_SuppressTypeBlocksDelivery(t *testing.T) {
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		SenderProfileID:    uuid.New(),
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	}
	settings := delivery.SettingsSnapshot{
		SuppressTypes: []delivery.NotificationType{delivery.TypeNewMessage},
	}
	out := delivery.ApplySettings(baseOnlineDecision(), in, settings)
	require.False(t, out.Push)
	require.False(t, out.InApp)
}

func TestApplySettings_MutedChatDoesNotSuppressMentionWithoutOverride(t *testing.T) {
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		SenderProfileID:    uuid.New(),
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeMention,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	}
	settings := delivery.SettingsSnapshot{
		ChatMuted:            true,
		MentionOverridesMute: false,
	}
	out := delivery.ApplySettings(baseOnlineDecision(), in, settings)
	require.True(t, out.Push, "chat mute alone only suppresses new_message, not mentions")
	require.True(t, out.InApp)
}

func TestApplySettings_MentionOverridesMute(t *testing.T) {
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		SenderProfileID:    uuid.New(),
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeMention,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	}
	settings := delivery.SettingsSnapshot{
		ChatMuted:            true,
		MentionOverridesMute: true,
	}
	out := delivery.ApplySettings(baseOnlineDecision(), in, settings)
	require.True(t, out.Push, "@mention should override chat mute")
	require.True(t, out.InApp)
}
