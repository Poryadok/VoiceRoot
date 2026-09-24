package consumer

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestValidateMessageConsumerRequiresExactProvisioning(t *testing.T) {
	valid := &nats.ConsumerInfo{Stream: jsStreamMessageEvents, Name: messageDurable, Config: nats.ConsumerConfig{
		Durable: messageDurable, FilterSubject: subjectMessageSent, DeliverSubject: messageDeliverSubject,
		DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy,
	}}
	require.NoError(t, validateMessageConsumer(valid))
	require.Error(t, validateMessageConsumer(nil))
	copy := *valid
	copy.Config.FilterSubject = "message.*"
	require.Error(t, validateMessageConsumer(&copy))
	copy = *valid
	copy.Config.DeliverSubject = "_INBOX.other"
	require.Error(t, validateMessageConsumer(&copy))
	copy = *valid
	copy.Config.AckPolicy = nats.AckNonePolicy
	require.Error(t, validateMessageConsumer(&copy))
}
