package apns_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func TestVoIPNoopSender_ReturnsNil(t *testing.T) {
	sender := &apns.VoIPNoopSender{}
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "t"}, push.Payload{})
	require.NoError(t, err)
}
