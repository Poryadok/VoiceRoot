package fcm_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
)

func TestStubSender_ReturnsErrNotImplemented(t *testing.T) {
	t.Parallel()

	var sender fcm.StubSender
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{}, fcm.PushPayload{})
	require.ErrorIs(t, err, fcm.ErrNotImplemented)
}

func TestFCMErrorStrings(t *testing.T) {
	t.Parallel()

	require.Equal(t, "fcm sender: not implemented", fcm.ErrNotImplemented.Error())
	require.Equal(t, "fcm: invalid token", fcm.ErrInvalidToken.Error())
}
