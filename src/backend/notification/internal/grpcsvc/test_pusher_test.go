package grpcsvc

import (
	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
)

func testPusher(fcmSender fcm.Sender, apnsSender apns.Sender) *dispatch.PushDispatcher {
	return &dispatch.PushDispatcher{FCM: fcmSender, APNs: apnsSender}
}
