package consumer

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	analyticsv1 "voice.app/voice/analytics/v1"
)

func jetStreamFixture(stream, consumer string, streamSequence uint64) *nats.Msg {
	// Metadata requires the same non-nil subscription binding that a delivered
	// JetStream message has; the reply is the protocol ACK subject used in production.
	return &nats.Msg{
		Sub:   &nats.Subscription{},
		Reply: fmt.Sprintf("$JS.ACK.%s.%s.1.%d.3.42.1", stream, consumer, streamSequence),
	}
}

func TestSourceDurableIsStableAcrossPodInstanceIDs(t *testing.T) {
	require.Equal(t, "analytics_v2_msg", analyticsDurableName("msg"))
	require.Equal(t, analyticsDurableName("msg"), analyticsQueueName("msg"))
	require.NotContains(t, analyticsDurableName("msg"), "hostname")
	require.NotContains(t, analyticsDurableName("msg"), "pod")
}

func TestNormalizeSourceEventIDRetainsValidEnvelopeID(t *testing.T) {
	id := uuid.NewString()
	require.Equal(t, id, normalizeSourceEventID(id, &nats.Msg{}))
}

func TestNormalizeSourceEventIDIsStableForSameJetStreamPosition(t *testing.T) {
	msg := jetStreamFixture("analytics_events", "analytics_v2_telemetry", 7)
	meta, err := msg.Metadata()
	require.NoError(t, err)
	require.Equal(t, "analytics_events", meta.Stream)
	require.Equal(t, uint64(7), meta.Sequence.Stream)
	first := normalizeSourceEventID("not-a-uuid", msg)
	second := normalizeSourceEventID("", msg)
	require.NotEmpty(t, first)
	require.Equal(t, first, second)
	require.NoError(t, uuid.Validate(first))
}

func TestAppendWithSourceAckUsesStableJetStreamID(t *testing.T) {
	event := &analyticsv1.AnalyticsEvent{EventId: "invalid"}
	msg := jetStreamFixture("message_events", "analytics_v2_msg", 7)
	require.NotEmpty(t, normalizeSourceEventID(event.GetEventId(), msg))
}
