package fcm_test

import (
	"context"
	"errors"
	"testing"

	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type stubMessagingClient struct {
	last *messaging.Message
	err  error
}

func (c *stubMessagingClient) Send(_ context.Context, message *messaging.Message) (string, error) {
	c.last = message
	return "msg-id", c.err
}

func TestHTTPSender_Success(t *testing.T) {
	client := &stubMessagingClient{}
	sender := fcm.NewHTTPSenderForTest(client)
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{
		Token:       "good-tok",
		PushService: "fcm",
	}, push.Payload{
		Title: "Match found",
		Body:  "Ready",
		Data:  map[string]string{"type": "match_found"},
	})
	require.NoError(t, err)
	require.Equal(t, "good-tok", client.last.Token)
	require.Equal(t, "match_found", client.last.Data["type"])
}

func TestHTTPSender_InvalidToken(t *testing.T) {
	client := &stubMessagingClient{
		err: errors.New("Requested entity was not found."),
	}
	sender := fcm.NewHTTPSenderForTest(client)
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "bad"}, push.Payload{Body: "hi"})
	require.ErrorIs(t, err, fcm.ErrInvalidToken)
}

func TestHTTPSender_OtherError(t *testing.T) {
	client := &stubMessagingClient{err: errors.New("network down")}
	sender := fcm.NewHTTPSenderForTest(client)
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "tok"}, push.Payload{Body: "hi"})
	require.Error(t, err)
	require.NotErrorIs(t, err, fcm.ErrInvalidToken)
}
