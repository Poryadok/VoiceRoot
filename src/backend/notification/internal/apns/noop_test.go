package apns_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func TestNoopSender_ReturnsNil(t *testing.T) {
	var nilSender *apns.NoopSender
	err := nilSender.Send(context.Background(), uuid.New(), store.DeviceToken{}, push.Payload{})
	require.NoError(t, err)

	sender := &apns.NoopSender{Logger: slog.Default()}
	err = sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "tok"}, push.Payload{
		Data: map[string]string{"type": "mention"},
	})
	require.NoError(t, err)
}
