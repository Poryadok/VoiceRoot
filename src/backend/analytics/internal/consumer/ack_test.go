package consumer

import (
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type ackSpy struct {
	acked bool
	naked bool
}

func (s *ackSpy) Ack(...nats.AckOpt) error {
	s.acked = true
	return nil
}

func (s *ackSpy) Nak(...nats.AckOpt) error {
	s.naked = true
	return nil
}

func TestJetStreamConsumeAckNakOnError(t *testing.T) {
	spy := &ackSpy{}
	jetStreamConsumeAck(spy, errors.New("boom"))
	require.True(t, spy.naked)
	require.False(t, spy.acked)
}

func TestJetStreamConsumeAckNoOpOnSuccess(t *testing.T) {
	spy := &ackSpy{}
	jetStreamConsumeAck(spy, nil)
	require.False(t, spy.acked)
	require.False(t, spy.naked)
}
