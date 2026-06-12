package dispatch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func TestMatchmakingPusher_NilSafe(t *testing.T) {
	var pusher *MatchmakingPusher
	require.NoError(t, pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		uuid.NewString(): {Push: true},
	}, push.Payload{}))
}

func TestMatchmakingPusher_SkipsNonPushRecipients(t *testing.T) {
	rec := &recordingFCMSender{}
	dispatcher := &PushDispatcher{FCM: rec}
	pusher := &MatchmakingPusher{Tokens: &store.DeviceTokenStore{}, Pusher: dispatcher}
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		uuid.NewString(): {Push: false, InApp: true},
		"bad-profile":    {Push: true},
	}, push.Payload{Body: "x"})
	require.NoError(t, err)
	require.Empty(t, rec.sent)
}
