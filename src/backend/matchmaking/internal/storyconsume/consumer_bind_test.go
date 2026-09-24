package storyconsume

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestStoryDurableConfig(t *testing.T) {
	valid := &nats.ConsumerInfo{Stream: streamName, Name: defaultDurable, Config: nats.ConsumerConfig{
		Durable: defaultDurable, FilterSubjects: []string{subjectStoryLfpCreated, subjectStoryLfpResponse},
		DeliverSubject: storyDeliverySubject, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy,
	}}
	require.NoError(t, validateStoryDurable(valid))
	invalid := *valid
	invalid.Config.FilterSubject = "story.>"
	invalid.Config.FilterSubjects = nil
	require.Error(t, validateStoryDurable(&invalid))
	invalid = *valid
	invalid.Config.FilterSubjects = []string{subjectStoryLfpCreated, "story.deleted"}
	require.Error(t, validateStoryDurable(&invalid))
	invalid = *valid
	invalid.Config.AckPolicy = nats.AckNonePolicy
	require.Error(t, validateStoryDurable(&invalid))
	require.Error(t, validateStoryDurable(nil))
}

func TestStoryConsumerUsesScopedNATSInbox(t *testing.T) {
	var opts nats.Options
	for _, option := range storyNATSOptions() {
		require.NoError(t, option(&opts))
	}
	require.Equal(t, "_INBOX.voice.matchmaking", opts.InboxPrefix)
}
