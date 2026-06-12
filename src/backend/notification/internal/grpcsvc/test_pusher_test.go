package grpcsvc

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type recordingVoIPSender struct {
	sent []push.Payload
}

func (r *recordingVoIPSender) Send(_ context.Context, _ uuid.UUID, _ store.DeviceToken, payload push.Payload) error {
	r.sent = append(r.sent, payload)
	return nil
}

func testPusher(fcmSender fcm.Sender, apnsSender apns.Sender) *dispatch.PushDispatcher {
	return &dispatch.PushDispatcher{FCM: fcmSender, APNs: apnsSender}
}

func testPusherWithVoIP(fcmSender fcm.Sender, apnsSender apns.Sender, voipSender apns.VoIPSender) *dispatch.PushDispatcher {
	return &dispatch.PushDispatcher{FCM: fcmSender, APNs: apnsSender, VoIP: voipSender}
}
