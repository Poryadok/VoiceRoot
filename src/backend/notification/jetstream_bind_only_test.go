package main

import (
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/consumer"
)

func TestBindPreprovisionedConsumerUsesOnlyExistingDurable(t *testing.T) {
	sentinel := errors.New("consumer is missing")
	var gotSubject string
	_, err := bindPreprovisionedSubscribe(
		func(subject string, _ nats.MsgHandler, _ ...nats.SubOpt) (*nats.Subscription, error) {
			gotSubject = subject
			return nil, sentinel
		},
		jsStreamMessageEvents,
		consumer.SharedDurable("message"),
		func(*nats.Msg) {},
		nats.ManualAck(),
	)
	require.ErrorIs(t, err, sentinel)
	require.Empty(t, gotSubject, "bind-only consumers must not declare a subject route at runtime")
}
