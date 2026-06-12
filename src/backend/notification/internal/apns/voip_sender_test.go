package apns_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func TestHTTPVoIPSender_Success(t *testing.T) {
	client := &stubAPNSClient{resp: &apns2.Response{StatusCode: 200}}
	sender := apns.NewHTTPVoIPSenderForTest(client, "com.voice.app")

	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{
		Token:       "voip-tok",
		PushService: "voip_apns",
	}, push.Payload{
		Data: map[string]string{"type": "incoming_call", "room_id": "r1"},
	})
	require.NoError(t, err)
	require.NotNil(t, client.last)
	require.Equal(t, "com.voice.app.voip", client.last.Topic)
	require.Equal(t, apns2.PushTypeVOIP, client.last.PushType)
}

func TestHTTPVoIPSender_Unavailable(t *testing.T) {
	var sender *apns.HTTPVoIPSender
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "t"}, push.Payload{})
	require.Error(t, err)
}

func TestHTTPVoIPSender_RejectedNonInvalidToken(t *testing.T) {
	client := &stubAPNSClient{
		resp: &apns2.Response{StatusCode: 400, Reason: apns2.ReasonBadTopic},
	}
	sender := apns.NewHTTPVoIPSenderForTest(client, "com.voice.app")
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "tok"}, push.Payload{
		Data: map[string]string{"type": "incoming_call"},
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, apns.ErrInvalidToken)
}

func TestHTTPVoIPSender_InvalidToken(t *testing.T) {
	client := &stubAPNSClient{
		resp: &apns2.Response{StatusCode: 410, Reason: apns2.ReasonUnregistered},
	}
	sender := apns.NewHTTPVoIPSenderForTest(client, "com.voice.app")
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "bad"}, push.Payload{
		Data: map[string]string{"type": "incoming_call"},
	})
	require.ErrorIs(t, err, apns.ErrInvalidToken)
}
