package delivery_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
)

func TestDecideRouting_OnlineNoPush(t *testing.T) {
	recipient := uuid.New()
	sender := uuid.New()
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: recipient,
		SenderProfileID:    sender,
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           true,
		At:                 time.Now().UTC(),
	})
	require.True(t, decision.InApp, "online users receive in-app notifications")
	require.False(t, decision.Push, "online users must not receive push")
}

func TestDecideRouting_OfflinePush(t *testing.T) {
	recipient := uuid.New()
	sender := uuid.New()
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: recipient,
		SenderProfileID:    sender,
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	})
	require.True(t, decision.InApp)
	require.True(t, decision.Push, "offline users receive push for new_message")
}

func TestDecideRouting_SenderExcluded(t *testing.T) {
	sender := uuid.New()
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: sender,
		SenderProfileID:    sender,
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	})
	require.False(t, decision.InApp)
	require.False(t, decision.Push, "sender must not receive own notification")
}
