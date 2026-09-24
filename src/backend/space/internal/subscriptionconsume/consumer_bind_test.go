package subscriptionconsume

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestSpaceEntitlementDurableConfig(t *testing.T) {
	valid := &nats.ConsumerInfo{Stream: streamName, Name: defaultDurable, Config: nats.ConsumerConfig{
		Durable: defaultDurable, FilterSubjects: []string{subjectSpaceProStarted, subjectSpaceProExpired},
		DeliverSubject: spaceDeliverySubject, DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy,
	}}
	require.NoError(t, validateSpaceDurable(valid))
	canonical := *valid
	canonical.Config.FilterSubjects = []string{subjectSpaceProExpired, subjectSpaceProStarted}
	require.NoError(t, validateSpaceDurable(&canonical), "NATS may return filter_subjects in canonical order")
	invalid := *valid
	invalid.Config.FilterSubjects = []string{subjectSpaceProStarted, "subscription.user_premium_started"}
	require.Error(t, validateSpaceDurable(&invalid))
	invalid = *valid
	invalid.Config.FilterSubjects = []string{subjectSpaceProStarted, subjectSpaceProStarted}
	require.Error(t, validateSpaceDurable(&invalid), "duplicate filter must not replace the expired subject")
	invalid = *valid
	invalid.Config.FilterSubjects = nil
	invalid.Config.FilterSubject = "subscription.>"
	require.Error(t, validateSpaceDurable(&invalid))
	invalid = *valid
	invalid.Config.AckPolicy = nats.AckNonePolicy
	require.Error(t, validateSpaceDurable(&invalid))
	require.Error(t, validateSpaceDurable(nil))
}

func TestSpaceEntitlementUsesScopedNATSInbox(t *testing.T) {
	var opts nats.Options
	for _, option := range entitlementNATSOptions() {
		require.NoError(t, option(&opts))
	}
	require.Equal(t, "_INBOX.voice.space", opts.InboxPrefix)
}
