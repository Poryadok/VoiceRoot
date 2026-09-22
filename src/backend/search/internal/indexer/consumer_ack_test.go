package indexer

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type stubJetStreamMsg struct {
	*nats.Msg
	acked    bool
	nacked   bool
	termed   bool
	nakDelay time.Duration
	ackErr   error
	nakErr   error
	termErr  error
}

func (s *stubJetStreamMsg) Ack(...nats.AckOpt) error {
	s.acked = true
	return s.ackErr
}

func (s *stubJetStreamMsg) Nak(...nats.AckOpt) error {
	s.nacked = true
	return s.nakErr
}

func (s *stubJetStreamMsg) NakWithDelay(delay time.Duration, _ ...nats.AckOpt) error {
	s.nacked = true
	s.nakDelay = delay
	return s.nakErr
}

func (s *stubJetStreamMsg) Term(...nats.AckOpt) error {
	s.termed = true
	return s.termErr
}

func TestJetStreamConsumeAck_SuccessAcks(t *testing.T) {
	t.Parallel()
	msg := &stubJetStreamMsg{}
	jetStreamConsumeAck(msg, nil, "message", nil, nil)
	require.True(t, msg.acked)
	require.False(t, msg.nacked)
}

func TestJetStreamConsumeAck_HandlerErrorNaks(t *testing.T) {
	t.Parallel()
	msg := &stubJetStreamMsg{}
	jetStreamConsumeAck(msg, errTestConsume, "message", nil, nil)
	require.False(t, msg.acked)
	require.True(t, msg.nacked)
	require.Positive(t, msg.nakDelay)
}

func TestJetStreamConsumeAck_DurableMutationSuccessAcks(t *testing.T) {
	t.Parallel()
	msg := jetStreamDeliveryFixture(2)
	mutated := false

	consumeJetStreamMessage(msg, "message", nil, nil, func() error {
		mutated = true
		return nil
	})

	require.True(t, mutated)
	require.True(t, msg.acked)
	require.False(t, msg.nacked)
	require.False(t, msg.termed)
}

func TestJetStreamConsumeAck_TransientFailureNaksAndRemainsPending(t *testing.T) {
	t.Parallel()
	msg := jetStreamDeliveryFixture(1)
	registry := prometheus.NewRegistry()
	metrics := newConsumerMetrics(registry)

	jetStreamConsumeAck(msg, errTestConsume, "message", metrics, nil)

	require.True(t, msg.nacked)
	require.False(t, msg.termed)
	require.Equal(t, time.Second, msg.nakDelay)
	require.Equal(t, float64(1), consumerOutcomeValue(t, registry, "message", consumerOutcomeRetry))
}

func TestJetStreamConsumeAck_FailedAckRecordsOnlyDispositionFailure(t *testing.T) {
	t.Parallel()
	msg := &stubJetStreamMsg{ackErr: errTestConsume}
	registry := prometheus.NewRegistry()
	metrics := newConsumerMetrics(registry)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	jetStreamConsumeAck(msg, nil, "message_events", metrics, logger)

	require.Equal(t, float64(0), consumerOutcomeValue(t, registry, "message_events", consumerOutcomeAck))
	require.Equal(t, float64(1), consumerOutcomeValue(t, registry, "message_events", consumerOutcomeDispositionFailure))
	require.Contains(t, logs.String(), "outcome=ack")
}

func TestJetStreamConsumeAck_ReplayedTransientFailureStillNaks(t *testing.T) {
	t.Parallel()
	msg := jetStreamDeliveryFixture(2)
	registry := prometheus.NewRegistry()
	metrics := newConsumerMetrics(registry)

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	jetStreamConsumeAck(msg, errTestConsume, "message", metrics, logger)

	require.True(t, msg.nacked)
	require.False(t, msg.termed)
	require.Equal(t, 5*time.Second, msg.nakDelay)
	require.Empty(t, logs.String())
	require.Equal(t, float64(1), consumerOutcomeValue(t, registry, "message", consumerOutcomeRetry))
}

func TestJetStreamConsumeAck_PermanentSemanticPoisonTerminatesAndObservesIt(t *testing.T) {
	t.Parallel()
	msg := &stubJetStreamMsg{}
	registry := prometheus.NewRegistry()
	metrics := newConsumerMetrics(registry)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	jetStreamConsumeAck(msg, newPermanentConsumeError("invalid message_id"), "message_events", metrics, logger)

	require.False(t, msg.nacked)
	require.True(t, msg.termed)
	require.Equal(t, float64(1), consumerOutcomeValue(t, registry, "message_events", consumerOutcomeTerminalSemanticPoison))
	require.Contains(t, logs.String(), "outcome="+consumerOutcomeTerminalSemanticPoison)
}

func TestSearchConsumerConfigBoundsTransientRedelivery(t *testing.T) {
	t.Parallel()
	config := searchConsumerConfig("search_msg_v3_test", "message.>")

	require.Equal(t, nats.AckExplicitPolicy, config.AckPolicy)
	require.Equal(t, nats.DeliverAllPolicy, config.DeliverPolicy)
	require.Equal(t, -1, config.MaxDeliver)
	require.Equal(t, searchConsumerRetryBackoff, config.BackOff)
}

func TestReconcileSearchConsumerConfig_UpdatesDifferentBackoffBeforeBinding(t *testing.T) {
	t.Parallel()
	config := searchConsumerConfig("search_msg_v3_test", "message.>")
	config.BackOff = []time.Duration{time.Second}

	reconciled, update, err := reconcileSearchConsumerConfig(config, "message.>")

	require.NoError(t, err)
	require.True(t, update)
	require.Equal(t, searchConsumerRetryBackoff, reconciled.BackOff)
	require.Equal(t, searchConsumerMaxDeliver, reconciled.MaxDeliver)
}

func TestReconcileSearchConsumerConfig_RejectsIncompatibleConfigBeforeBinding(t *testing.T) {
	t.Parallel()
	config := searchConsumerConfig("search_msg_v3_test", "message.>")
	config.AckPolicy = nats.AckNonePolicy

	_, update, err := reconcileSearchConsumerConfig(config, "message.>")

	require.Error(t, err)
	require.False(t, update)
}

func TestPrepareThenBindSearchConsumer_IncompatibleConfigPreventsEarlyBinding(t *testing.T) {
	t.Parallel()
	bound := false

	_, err := prepareThenBindSearchConsumer(
		func() error { return errTestConsume },
		func() (*nats.Subscription, error) {
			bound = true
			return nil, nil
		},
	)

	require.Error(t, err)
	require.False(t, bound)
}

func TestMessageEventsJetStreamSubjectPrefix(t *testing.T) {
	t.Parallel()
	require.Equal(t, "message.>", jsSubjectMessageEvents)
	require.Equal(t, "search_msg_v3_", jsDurableMessagePrefix)
}

func TestJetStreamTermAck_Terminates(t *testing.T) {
	t.Parallel()
	msg := &stubJetStreamMsg{}
	jetStreamTermAck(msg, "message", nil, nil)
	require.True(t, msg.termed)
}

var errTestConsume = &consumeTestError{}

type consumeTestError struct{}

func (e *consumeTestError) Error() string { return "consume failed" }

func jetStreamDeliveryFixture(deliveries int) *stubJetStreamMsg {
	return &stubJetStreamMsg{Msg: &nats.Msg{
		Sub:   &nats.Subscription{},
		Reply: fmt.Sprintf("$JS.ACK.message_events.search_msg_v3_test.%d.7.3.42.1", deliveries),
	}}
}

func consumerOutcomeValue(t *testing.T, registry *prometheus.Registry, stream, outcome string) float64 {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "search_indexer_consume_outcomes_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			var gotStream, gotOutcome string
			for _, label := range metric.GetLabel() {
				switch label.GetName() {
				case "stream":
					gotStream = label.GetValue()
				case "outcome":
					gotOutcome = label.GetValue()
				}
			}
			if gotStream == stream && gotOutcome == outcome {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}
