package voiceevents

import (
	"context"

	eventsv1 "voice.app/voice/events/v1"
)

type Publisher interface {
	PublishCallIncoming(context.Context, *eventsv1.CallIncoming) error
	PublishCallAccepted(context.Context, *eventsv1.CallAccepted) error
	PublishCallDeclined(context.Context, *eventsv1.CallDeclined) error
	PublishCallMissed(context.Context, *eventsv1.CallMissed) error
	PublishCallEnded(context.Context, *eventsv1.CallEnded) error
	PublishVoiceStateChanged(context.Context, *eventsv1.VoiceStateChanged) error
}
