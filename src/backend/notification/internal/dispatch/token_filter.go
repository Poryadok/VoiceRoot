package dispatch

// ShouldDeliverPushToToken returns whether a token should receive a given notification type.
func ShouldDeliverPushToToken(notificationType, pushService string) bool {
	if notificationType == "incoming_call" {
		return pushService == "voip_apns"
	}
	return pushService != "voip_apns"
}
