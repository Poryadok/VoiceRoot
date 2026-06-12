package dispatch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func TestPushDispatcher_UnavailableSenders(t *testing.T) {
	d := &PushDispatcher{}
	payload := push.Payload{Body: "x"}
	err := d.Send(context.Background(), uuid.New(), store.DeviceToken{PushService: "fcm"}, payload)
	require.Error(t, err)
	err = d.Send(context.Background(), uuid.New(), store.DeviceToken{PushService: "apns"}, payload)
	require.Error(t, err)
}

func TestPushDispatcher_NilDispatcher(t *testing.T) {
	var d *PushDispatcher
	err := d.Send(context.Background(), uuid.New(), store.DeviceToken{}, push.Payload{})
	require.Error(t, err)
}
