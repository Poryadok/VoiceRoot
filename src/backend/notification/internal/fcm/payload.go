package fcm

import (
	"fmt"

	"firebase.google.com/go/v4/messaging"

	"voice/backend/notification/internal/push"
)

// BuildFCMMessage maps a neutral push payload to an FCM v1 message.
func BuildFCMMessage(deviceToken string, in push.Payload) (*messaging.Message, error) {
	if deviceToken == "" {
		return nil, fmt.Errorf("fcm: device token required")
	}
	data := make(map[string]string, len(in.Data))
	for k, v := range in.Data {
		data[k] = v
	}
	body := displayBody(in)
	msg := &messaging.Message{
		Token: deviceToken,
		Data:  data,
	}
	if in.Title != "" || body != "" {
		msg.Notification = &messaging.Notification{
			Title: in.Title,
			Body:  body,
		}
	}
	if in.CollapseTag != "" || in.Silent {
		msg.Android = &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				Tag:               in.CollapseTag,
				NotificationCount: nil,
			},
		}
		if in.CollapseTag != "" {
			msg.Android.CollapseKey = in.CollapseTag
		}
	}
	if in.CollapseTag != "" || in.Silent {
		msg.Webpush = &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title:  in.Title,
				Body:   body,
				Silent: in.Silent,
			},
		}
		if in.CollapseTag != "" {
			msg.Webpush.Headers = map[string]string{"Topic": in.CollapseTag}
		}
	}
	return msg, nil
}

func displayBody(in push.Payload) string {
	if in.Counter > 1 && in.Body != "" {
		return fmt.Sprintf("%s and %d more messages", in.Body, in.Counter-1)
	}
	return in.Body
}
