package spaceevents

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
	require.Equal(t, []string{
		"chat.created", "chat.member_changed", "chat.dm_peer_deleted",
		"space.tree_changed", "space.created", "voice.room_created", "voice.room_deleted",
		"space.invite_created", "space.member_joined", "space.member_left", "space.updated", "space.deleted",
	}, spaceEventStreamSubjects())

	valid := &nats.StreamInfo{Config: nats.StreamConfig{Name: streamName, Subjects: spaceEventStreamSubjects(), Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.FileStorage}}
	require.NoError(t, validateBootstrappedStream(valid))
	require.Error(t, validateBootstrappedStream(nil))
	valid.Config.Subjects = valid.Config.Subjects[:len(valid.Config.Subjects)-1]
	require.Error(t, validateBootstrappedStream(valid))
}

func TestPublisherFailsClosedWhenStreamIsUnavailable(t *testing.T) {
	publisher := &JetStreamPublisher{js: unavailableJetStream{}}
	require.Error(t, publisher.PublishSpaceCreated(t.Context(), "space", "owner"))
}
