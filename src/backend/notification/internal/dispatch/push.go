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
	FCM               fcm.Sender
	APNs              apns.Sender
	VoIP              apns.VoIPSender
	LifecycleDelivery interface {
		WithChatDelivery(context.Context, string, func(context.Context) error) error
		WithSpaceDelivery(context.Context, string, func(context.Context) error) error
	}
}

// Send delivers a push to the appropriate sender for the device token.
func (d *PushDispatcher) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload push.Payload) error {
	if d == nil {
		return fmt.Errorf("push dispatcher unavailable")
	}
	if d.LifecycleDelivery != nil {
		dispatch := func(guarded context.Context) error { return d.send(guarded, profileID, token, payload) }
		spaceDispatch := func(guarded context.Context) error {
			if id := payload.Data["space_id"]; id != "" {
				return d.LifecycleDelivery.WithSpaceDelivery(guarded, id, dispatch)
			}
			return dispatch(guarded)
		}
		if id := payload.Data["chat_id"]; id != "" {
			return d.LifecycleDelivery.WithChatDelivery(ctx, id, spaceDispatch)
		}
		return spaceDispatch(ctx)
	}
	return d.send(ctx, profileID, token, payload)
}

func (d *PushDispatcher) send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload push.Payload) error {
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
