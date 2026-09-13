package messageevents

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	eventsv1 "voice.app/voice/events/v1"
)

func TestMessageSentSchedulingFieldsKeepReservedNumbers(t *testing.T) {
	descriptor := (&eventsv1.MessageSent{}).ProtoReflect().Descriptor()
	require.Equal(t, protoreflect.FieldNumber(9), descriptor.Fields().ByName("was_scheduled").Number())
	scheduledAt := descriptor.Fields().ByName("scheduled_at")
	require.Equal(t, protoreflect.FieldNumber(10), scheduledAt.Number())
	require.True(t, scheduledAt.HasOptionalKeyword())
}
