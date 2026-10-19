package socialprincipal

import "testing"

func TestMessagingReportCapabilityIsExact(t *testing.T) {
	const capability = "social-messaging-report"
	const method = "/voice.social.v1.SocialService/IsProfilePairsBlocked"
	if Method(capability) != method || !AllowsMethod(capability, method) || expectedIssuer(capability) != "messaging" || !RequiresMutualTLS(capability) {
		t.Fatal("missing exact protected report capability")
	}
	for _, other := range []string{"/voice.social.v1.SocialService/IsProfilePairBlocked", "/voice.social.v1.SocialService/BlockAccount", "/voice.user.v1.UserService/GetScheduledMessageDispatchPresence", "/voice.space.v1.SpaceService/ResolveMemberCommunicationState"} {
		if AllowsMethod(capability, other) {
			t.Fatalf("report capability admits %s", other)
		}
	}
	for _, other := range []string{"user", "space", "file", "messaging", "space-messaging-communication", "notification"} {
		if AllowsMethod(other, method) {
			t.Fatalf("other capability %s admits report method", other)
		}
	}
}
