package apns

import (
	"encoding/json"
	"fmt"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"

	"voice/backend/notification/internal/push"
)

// BuildNotification maps a neutral push payload to an APNs notification.
func BuildNotification(bundleID, deviceToken string, in push.Payload) (*apns2.Notification, error) {
	if deviceToken == "" {
		return nil, fmt.Errorf("apns: device token required")
	}
	p := payload.NewPayload()
	if in.Title != "" || in.Body != "" {
		p = p.AlertTitle(in.Title).AlertBody(displayBody(in))
	} else if in.Body != "" {
		p = p.Alert(displayBody(in))
	}
	for k, v := range in.Data {
		p = p.Custom(k, v)
	}
	if in.CollapseTag != "" {
		p = p.ThreadID(in.CollapseTag)
	}
	n := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       bundleID,
		Payload:     p,
	}
	if in.CollapseTag != "" {
		n.CollapseID = in.CollapseTag
	}
	return n, nil
}

func displayBody(in push.Payload) string {
	if in.Counter > 1 && in.Body != "" {
		return fmt.Sprintf("%s and %d more messages", in.Body, in.Counter-1)
	}
	return in.Body
}

// PayloadJSON returns the JSON bytes of the APNs payload for tests and debugging.
func PayloadJSON(n *apns2.Notification) ([]byte, error) {
	if n == nil || n.Payload == nil {
		return nil, fmt.Errorf("apns: nil notification")
	}
	return json.Marshal(n.Payload)
}
