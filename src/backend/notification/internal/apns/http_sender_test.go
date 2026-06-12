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

type stubAPNSClient struct {
	last    *apns2.Notification
	resp    *apns2.Response
	err     error
}

func (c *stubAPNSClient) PushWithContext(_ apns2.Context, n *apns2.Notification) (*apns2.Response, error) {
	c.last = n
	return c.resp, c.err
}

func TestHTTPSender_InvalidTokenReason(t *testing.T) {
	client := &stubAPNSClient{
		resp: &apns2.Response{StatusCode: 410, Reason: apns2.ReasonUnregistered},
	}
	sender := apns.NewHTTPSenderForTest(client, "com.voice.app")

	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{
		Token:       "bad-tok",
		PushService: "apns",
	}, push.Payload{Body: "hi", Data: map[string]string{"type": "new_message"}})
	require.ErrorIs(t, err, apns.ErrInvalidToken)
	require.NotNil(t, client.last)
	require.Equal(t, "bad-tok", client.last.DeviceToken)
}

func TestHTTPSender_RejectedNonInvalidToken(t *testing.T) {
	client := &stubAPNSClient{
		resp: &apns2.Response{StatusCode: 400, Reason: apns2.ReasonBadTopic},
	}
	sender := apns.NewHTTPSenderForTest(client, "com.voice.app")
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "tok"}, push.Payload{Body: "hi"})
	require.Error(t, err)
	require.NotErrorIs(t, err, apns.ErrInvalidToken)
}

func TestHTTPSender_Success(t *testing.T) {
	client := &stubAPNSClient{resp: &apns2.Response{StatusCode: 200}}
	sender := apns.NewHTTPSenderForTest(client, "com.voice.app")

	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "good-tok"}, push.Payload{
		Title: "Match found",
		Body:  "Ready",
		Data:  map[string]string{"type": "match_found"},
	})
	require.NoError(t, err)
	require.Equal(t, "com.voice.app", client.last.Topic)
}
