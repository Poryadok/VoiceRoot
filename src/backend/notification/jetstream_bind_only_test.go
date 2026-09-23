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
		jsSubjectMessageEvents,
		func(*nats.Msg) {},
		nats.ManualAck(),
	)
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, jsSubjectMessageEvents, gotSubject, "bind-only consumers must bind their exact fixed filter")
}

func TestNotificationConsumerRequiresExactPreprovisionedFilter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter string
		multi  []string
		wantOK bool
	}{
		{name: "exact", filter: jsSubjectMessageEvents, wantOK: true},
		{name: "unfiltered durable"},
		{name: "broader wildcard", filter: ">"},
		{name: "other stream subject", filter: "social.>"},
		{name: "multiple filters", multi: []string{jsSubjectMessageEvents, "social.>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &nats.ConsumerInfo{Stream: jsStreamMessageEvents, Name: consumer.SharedDurable("message"), Config: nats.ConsumerConfig{
				FilterSubject: tc.filter, FilterSubjects: tc.multi,
			}}
			err := validateNotificationConsumerFilter(info, jsStreamMessageEvents, consumer.SharedDurable("message"), jsSubjectMessageEvents)
			if tc.wantOK {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
