package botevents

import (
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type unavailableJetStream struct{}

func (unavailableJetStream) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, errors.New("missing")
}
func (unavailableJetStream) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) {
	panic("publish must not run")
}

func TestValidateBootstrappedStream(t *testing.T) {
	valid := &nats.StreamInfo{Config: nats.StreamConfig{Name: streamName, Subjects: botEventStreamSubjects(), Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.FileStorage}}
	require.NoError(t, validateBootstrappedStream(valid))
	require.Error(t, validateBootstrappedStream(nil))
	valid.Config.Subjects = valid.Config.Subjects[:len(valid.Config.Subjects)-1]
	require.Error(t, validateBootstrappedStream(valid))
}

func TestPublisherFailsClosedWhenStreamIsUnavailable(t *testing.T) {
	publisher := &JetStreamPublisher{js: unavailableJetStream{}}
	require.Error(t, publisher.PublishBotRegistered(t.Context(), "bot", "owner"))
}

func TestBotPublisherReplyInboxUsesScopedPrefix(t *testing.T) {
	options := nats.GetDefaultOptions()
	for _, option := range botNatsOptions() {
		require.NoError(t, option(&options))
	}
	require.Equal(t, "_INBOX.voice.bot", options.InboxPrefix)
}
