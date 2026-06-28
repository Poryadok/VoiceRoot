package dispatch

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

// PushDispatcher routes push delivery to FCM, APNs, or VoIP APNs based on device token metadata.
type PushDispatcher struct {
	FCM  fcm.Sender
	APNs apns.Sender
	VoIP apns.VoIPSender
}

// Send delivers a push to the appropriate sender for the device token.
func (d *PushDispatcher) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload push.Payload) error {
	if d == nil {
		return fmt.Errorf("push dispatcher unavailable")
	}
	fcmPayload := fcm.PushPayload(payload)
	switch token.PushService {
	case "apns":
		if d.APNs == nil {
			return fmt.Errorf("apns sender unavailable")
		}
		return d.APNs.Send(ctx, profileID, token, payload)
	case "voip_apns":
		if d.VoIP == nil {
			return fmt.Errorf("apns voip sender unavailable")
		}
		return d.VoIP.Send(ctx, profileID, token, payload)
	default:
		if d.FCM == nil {
			return fmt.Errorf("fcm sender unavailable")
		}
		return d.FCM.Send(ctx, profileID, token, fcmPayload)
	}
}
