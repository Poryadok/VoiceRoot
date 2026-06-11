package fcm_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
)

func TestNoopSender_SendReturnsNil(t *testing.T) {
	t.Parallel()

	var nilSender *fcm.NoopSender
	err := nilSender.Send(context.Background(), uuid.New(), store.DeviceToken{}, fcm.PushPayload{
		Data: map[string]string{"type": "new_message"},
	})
	require.NoError(t, err, "noop sender must not fail when FCM is not configured")

	sender := &fcm.NoopSender{Logger: slog.Default()}
	err = sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "tok"}, fcm.PushPayload{
		Data: map[string]string{"type": "mention"},
	})
	require.NoError(t, err)
}
