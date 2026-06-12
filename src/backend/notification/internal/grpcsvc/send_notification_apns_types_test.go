package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/store"

	notificationv1 "voice.app/voice/notification/v1"
)

func TestSendNotification_RoutesInScopeTypesToAPNS(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	types := []string{
		"new_message",
		"mention",
		"friend_request",
		"match_found",
		"system",
	}

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			tokens := &store.DeviceTokenStore{Pool: pool}
			profileID := uuid.New()
			_, err := tokens.Register(ctx, profileID, "ios", "ios-"+typ, "apns")
			require.NoError(t, err)

			apnsRec := &recordingAPNSSender{}
			svc := &NotificationGRPC{Pusher: testPusher(&recordingFCMSender{}, apnsRec), Tokens: tokens}
			_, err = svc.SendNotification(ctx, &notificationv1.SendNotificationRequest{
				ProfileId:        profileID.String(),
				NotificationType: typ,
				Body:             "body",
			})
			require.NoError(t, err)
			require.Len(t, apnsRec.sent, 1)
			require.Equal(t, typ, apnsRec.sent[0].Data["type"])
		})
	}
}

func TestSendNotification_SkipsVoIPAPNSToken(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	tokens := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()
	_, err := tokens.Register(ctx, profileID, "ios", "voip-tok", "voip_apns")
	require.NoError(t, err)

	apnsRec := &recordingAPNSSender{}
	fcmRec := &recordingFCMSender{}
	svc := &NotificationGRPC{Pusher: testPusher(fcmRec, apnsRec), Tokens: tokens}
	_, err = svc.SendNotification(ctx, &notificationv1.SendNotificationRequest{
		ProfileId:        profileID.String(),
		NotificationType: "incoming_call",
		Body:             "ring",
	})
	require.NoError(t, err)
	require.Empty(t, apnsRec.sent)
	require.Empty(t, fcmRec.sent)
}

