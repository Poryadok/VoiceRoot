package socialprincipal

import "testing"

func TestSpaceCommunicationCapabilitiesAreExact(t *testing.T) {
	methods := map[string]string{
		"space-voice-communication":     "/voice.space.v1.SpaceService/ResolveVoiceRoomAccess",
		"space-messaging-communication": "/voice.space.v1.SpaceService/ResolveMemberCommunicationState",
	}
	for capability, method := range methods {
		if Method(capability) != method || !AllowsMethod(capability, method) {
			t.Fatalf("protected capability %s missing", capability)
		}
		for other, otherMethod := range methods {
			if other != capability && AllowsMethod(capability, otherMethod) {
				t.Fatalf("%s grants other caller's method", capability)
			}
		}
		if AllowsMethod(capability, "/voice.space.v1.SpaceService/DeleteSpace") {
			t.Fatal("communication capability widened to mutation")
		}
	}
}
