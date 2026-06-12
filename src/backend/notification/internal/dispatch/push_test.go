package dispatch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type recordingFCMSender struct {
	sent []store.DeviceToken
}

func (r *recordingFCMSender) Send(_ context.Context, _ uuid.UUID, token store.DeviceToken, _ fcm.PushPayload) error {
	r.sent = append(r.sent, token)
	return nil
}

type recordingAPNSTokens struct {
	sent []store.DeviceToken
}

func (r *recordingAPNSTokens) Send(_ context.Context, _ uuid.UUID, token store.DeviceToken, _ push.Payload) error {
	r.sent = append(r.sent, token)
	return nil
}

func TestPushDispatcher_RoutesByPushService(t *testing.T) {
	fcmRec := &recordingFCMSender{}
	apnsRec := &recordingAPNSTokens{}
	d := &PushDispatcher{FCM: fcmRec, APNs: apnsRec}
	profileID := uuid.New()
	payload := push.Payload{Body: "hi", Data: map[string]string{"type": "new_message"}}

	require.NoError(t, d.Send(context.Background(), profileID, store.DeviceToken{
		Token:       "android-tok",
		PushService: "fcm",
	}, payload))
	require.NoError(t, d.Send(context.Background(), profileID, store.DeviceToken{
		Token:       "ios-tok",
		PushService: "apns",
	}, payload))
	require.NoError(t, d.Send(context.Background(), profileID, store.DeviceToken{
		Token:       "voip-tok",
		PushService: "voip_apns",
	}, payload))

	require.Len(t, fcmRec.sent, 1)
	require.Equal(t, "android-tok", fcmRec.sent[0].Token)
	require.Len(t, apnsRec.sent, 1)
	require.Equal(t, "ios-tok", apnsRec.sent[0].Token)
}

func TestPushDispatcher_ForwardsCollapseTagToAPNS(t *testing.T) {
	apnsRec := &recordingAPNSSender{}
	d := &PushDispatcher{APNs: apnsRec}
	payload := push.Payload{
		Body:        "hello",
		CollapseTag: "push:group:abc:chat-1",
		Counter:     2,
		Data:        map[string]string{"type": "new_message"},
	}
	require.NoError(t, d.Send(context.Background(), uuid.New(), store.DeviceToken{
		Token:       "ios",
		PushService: "apns",
	}, payload))
	require.Len(t, apnsRec.sent, 1)
	require.Equal(t, "push:group:abc:chat-1", apnsRec.sent[0].CollapseTag)
	require.Equal(t, 2, apnsRec.sent[0].Counter)
}

type recordingAPNSSender struct {
	sent []push.Payload
}

func (r *recordingAPNSSender) Send(_ context.Context, _ uuid.UUID, _ store.DeviceToken, payload push.Payload) error {
	r.sent = append(r.sent, payload)
	return nil
}

func TestPushDispatcher_InvalidTokenErrors(t *testing.T) {
	d := &PushDispatcher{
		FCM:  invalidFCM{},
		APNs: invalidAPNS{},
	}
	payload := push.Payload{Body: "x"}
	err := d.Send(context.Background(), uuid.New(), store.DeviceToken{PushService: "fcm"}, payload)
	require.ErrorIs(t, err, fcm.ErrInvalidToken)
	err = d.Send(context.Background(), uuid.New(), store.DeviceToken{PushService: "apns"}, payload)
	require.ErrorIs(t, err, apns.ErrInvalidToken)
}

type invalidFCM struct{}

func (invalidFCM) Send(context.Context, uuid.UUID, store.DeviceToken, push.Payload) error {
	return fcm.ErrInvalidToken
}

type invalidAPNS struct{}

func (invalidAPNS) Send(context.Context, uuid.UUID, store.DeviceToken, push.Payload) error {
	return apns.ErrInvalidToken
}
