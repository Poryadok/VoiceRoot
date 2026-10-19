package dispatch

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/releasepolicy"
)

// PushDispatcher routes push delivery to FCM, APNs, or VoIP APNs based on device token metadata.
type PushDispatcher struct {
	ReleasePolicy          *releasepolicy.Policy
	OnCapabilitySuppressed func(context.Context)
	FCM                    fcm.Sender
	APNs                   apns.Sender
	VoIP                   apns.VoIPSender
	LifecycleDelivery      interface {
		WithChatDelivery(context.Context, string, func(context.Context) error) error
		WithSpaceDelivery(context.Context, string, func(context.Context) error) error
	}
}

// Send delivers a push to the appropriate sender for the device token.
func (d *PushDispatcher) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload push.Payload) error {
	return d.PrepareAndSend(ctx, profileID, token, payload, func(context.Context) (push.Payload, bool, error) { return payload, true, nil })
}

// PrepareAndSend keeps ordinary admission and grouping inside the same lifecycle
// gate as external dispatch. The callback runs only when delivery is eligible.
func (d *PushDispatcher) PrepareAndSend(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload push.Payload, prepare func(context.Context) (push.Payload, bool, error)) error {
	if d == nil {
		return fmt.Errorf("push dispatcher unavailable")
	}
	send := func(guarded context.Context) error {
		allowed, err := d.releaseTargetAllowed(guarded, token)
		if err != nil || !allowed {
			return err
		}
		prepared, allowed, err := prepare(guarded)
		if err != nil || !allowed {
			return err
		}
		return d.send(guarded, profileID, token, prepared)
	}
	if d.LifecycleDelivery != nil {
		dispatch := send
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
	return send(ctx)
}

func (d *PushDispatcher) send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload push.Payload) error {
	allowed, err := d.releaseTargetAllowed(ctx, token)
	if err != nil || !allowed {
		return err
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

// releaseTargetAllowed is checked inside lifecycle protection before lazy quota
// and grouping preparation, then again at the final provider boundary.
func (d *PushDispatcher) releaseTargetAllowed(ctx context.Context, token store.DeviceToken) (bool, error) {
	if d.ReleasePolicy != nil && d.ReleasePolicy.Profile() == releasepolicy.Alpha {
		mobile, err := push.MobileTarget(token.Platform, token.PushService)
		if err != nil {
			return false, err
		}
		if mobile && !d.ReleasePolicy.Allows(releasepolicy.MobilePush) {
			if d.OnCapabilitySuppressed != nil {
				d.OnCapabilitySuppressed(ctx)
			}
			return false, nil
		}
	}
	return true, nil
}
