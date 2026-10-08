package spaceevents

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
		"space.invite_created", "space.member_joined", "space.member_left", "space.updated", "space.deleted", "space.deletion_scheduled", "space.restored", "space.voice_room_access_invalidated",
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

func TestComposeBootstrapStreamSupportsSpacePublisher(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../"))
	script, err := os.ReadFile(filepath.Join(root, "docker/nats/realtime-bootstrap.sh"))
	require.NoError(t, err)
	var subjects []string
	for _, line := range strings.Split(string(script), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 2 && fields[0] == "stream" && fields[1] == "chat_events" {
			subjects = fields[2:]
			break
		}
	}
	require.NotEmpty(t, subjects)
	server := startJSTestServer(t)
	connection, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(connection.Close)
	js, err := connection.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: subjects,
		Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.FileStorage})
	require.NoError(t, err)
	publisher, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })
	require.NoError(t, publisher.Validate(), "the central Compose bootstrap must support the actual Space publisher")
}
