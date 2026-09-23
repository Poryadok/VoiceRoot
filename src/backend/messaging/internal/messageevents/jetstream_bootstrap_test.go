package messageevents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type bootstrapJetStream struct {
	info      *nats.StreamInfo
	infoErr   error
	publishes int
}

var _ jetStreamClient = (*bootstrapJetStream)(nil)

func (j *bootstrapJetStream) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return j.info, j.infoErr
}

func (j *bootstrapJetStream) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) {
	j.publishes++
	return &nats.PubAck{Stream: streamName, Sequence: uint64(j.publishes)}, nil
}

func TestJetStreamPublisher_RejectsUnbootstrappedOrMismatchedStream(t *testing.T) {
	for _, tc := range []struct {
		name string
		js   *bootstrapJetStream
	}{
		{name: "exact", js: &bootstrapJetStream{info: &nats.StreamInfo{Config: nats.StreamConfig{Name: streamName, Subjects: messageEventStreamSubjects(), Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.FileStorage}}}},
		{name: "missing", js: &bootstrapJetStream{infoErr: errors.New("stream missing")}},
		{name: "mismatched", js: &bootstrapJetStream{info: &nats.StreamInfo{Config: nats.StreamConfig{Name: streamName, Subjects: []string{subjectMessageSent}}}}},
		{name: "extra subject", js: &bootstrapJetStream{info: &nats.StreamInfo{Config: nats.StreamConfig{Name: streamName, Subjects: append(messageEventStreamSubjects(), "message.unexpected"), Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.FileStorage}}}},
		{name: "wrong storage", js: &bootstrapJetStream{info: &nats.StreamInfo{Config: nats.StreamConfig{Name: streamName, Subjects: messageEventStreamSubjects(), Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.MemoryStorage}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publisher := &JetStreamPublisher{js: tc.js}

			err := publisher.PublishMessageSent(context.Background(), "message", "chat", "sender", false, "", false, "", false)

			if tc.name == "exact" {
				require.NoError(t, err)
				require.Equal(t, 1, tc.js.publishes)
				return
			}
			require.Error(t, err)
			require.Zero(t, tc.js.publishes)
		})
	}
}
