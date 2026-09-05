package userevents

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

const testSubjectAccountDeleted = "user.account_deleted"

func startEmbeddedUserJSTestServer(t *testing.T) *server.Server {
	t.Helper()

	s, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second), "embedded NATS server is not ready")
	t.Cleanup(s.Shutdown)
	return s
}

// TestJetStreamPublisher_AccountDeletedHasStreamPubAck is RED until the User
// publisher owns the Auth-published deletion subject in user_events.
func TestJetStreamPublisher_AccountDeletedHasStreamPubAck(t *testing.T) {
	server := startEmbeddedUserJSTestServer(t)

	pub, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pub.Close()) })
	require.NoError(t, pub.ensureStream())

	envelope, err := proto.Marshal(&eventsv1.UserStreamEvent{
		EventId:    "event-account-deleted-red",
		OccurredAt: timestamppb.New(time.Unix(1, 0).UTC()),
		Payload: &eventsv1.UserStreamEvent_UserAccountDeleted{
			UserAccountDeleted: &eventsv1.UserAccountDeleted{
				AccountId: "account-account-deleted-red",
			},
		},
	})
	require.NoError(t, err)

	ack, err := pub.js.PublishMsg(&nats.Msg{
		Subject: testSubjectAccountDeleted,
		Data:    envelope,
	})
	require.NoError(t, err, "the old exact subject list has no matching stream subject")
	require.NotNil(t, ack)
	require.Equal(t, streamName, ack.Stream)
}
