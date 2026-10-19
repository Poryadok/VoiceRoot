package apns

import (
	"fmt"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"

	"voice/backend/notification/internal/push"
)

// BuildVoIPNotification maps an incoming_call payload to an exact VoIP APNs topic.
func BuildVoIPNotification(topic, deviceToken string, in push.Payload) (*apns2.Notification, error) {
	if deviceToken == "" {
		return nil, fmt.Errorf("apns voip: device token required")
	}
	if topic == "" {
		return nil, fmt.Errorf("apns voip: topic required")
	}
	p := payload.NewPayload()
	for k, v := range in.Data {
		p = p.Custom(k, v)
	}
	return &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       topic,
		Payload:     p,
		PushType:    apns2.PushTypeVOIP,
		Priority:    apns2.PriorityHigh,
	}, nil
}
