package indexer

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type stubJetStreamMsg struct {
	acked  bool
	nacked bool
	termed bool
}

func (s *stubJetStreamMsg) Ack(...nats.AckOpt) error {
	s.acked = true
	return nil
}

func (s *stubJetStreamMsg) Nak(...nats.AckOpt) error {
	s.nacked = true
	return nil
}

func (s *stubJetStreamMsg) Term(...nats.AckOpt) error {
	s.termed = true
	return nil
}

func TestJetStreamConsumeAck_SuccessAcks(t *testing.T) {
	t.Parallel()
	msg := &stubJetStreamMsg{}
	jetStreamConsumeAck(msg, nil)
	require.True(t, msg.acked)
	require.False(t, msg.nacked)
}

func TestJetStreamConsumeAck_HandlerErrorNaks(t *testing.T) {
	t.Parallel()
	msg := &stubJetStreamMsg{}
	jetStreamConsumeAck(msg, errTestConsume)
	require.False(t, msg.acked)
	require.True(t, msg.nacked)
}

func TestJetStreamTermAck_Terminates(t *testing.T) {
	t.Parallel()
	msg := &stubJetStreamMsg{}
	jetStreamTermAck(msg)
	require.True(t, msg.termed)
}

var errTestConsume = &consumeTestError{}

type consumeTestError struct{}

func (e *consumeTestError) Error() string { return "consume failed" }
