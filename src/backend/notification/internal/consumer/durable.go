package consumer

import "strings"

const (
	durableMessage      = "notif_msg"
	durableMatchmaking  = "notif_mm"
	durableVoice        = "notif_voice"
	durableStory        = "notif_story"
	durableSocial       = "notif_social"
	durableSubscription = "notif_subscription"
)

// SharedDurable returns the cluster-wide JetStream durable name for notification consumers.
// All replicas share one durable so each message is delivered to a single pod (work-queue).
func SharedDurable(stream string) string {
	switch strings.TrimSpace(stream) {
	case "message":
		return durableMessage
	case "matchmaking":
		return durableMatchmaking
	case "voice":
		return durableVoice
	case "story":
		return durableStory
	case "social":
		return durableSocial
	case "subscription":
		return durableSubscription
	default:
		return "notif_" + strings.TrimSpace(stream)
	}
}
