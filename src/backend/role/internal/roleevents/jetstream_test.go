package roleevents

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func startRoleJSTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	t.Cleanup(func() { s.Shutdown() })
	return s
}

func TestNewJetStreamPublisher_EmptyURL(t *testing.T) {
	_, err := NewJetStreamPublisher("")
	require.Error(t, err)
}

func TestStreamHasSubject_RoleEvents(t *testing.T) {
	t.Parallel()
	require.False(t, streamHasSubject(nil, subjectRoleCreated))
	info := &nats.StreamInfo{Config: nats.StreamConfig{Subjects: []string{"other.event"}}}
	require.False(t, streamHasSubject(info, subjectRoleAssigned))
	info.Config.Subjects = append(info.Config.Subjects, subjectRoleAssigned)
	require.True(t, streamHasSubject(info, subjectRoleAssigned))
}

// TestJetStreamPublisher_RoleCreatedRoundTrip documents role-service.md role.created on role.events.
func TestJetStreamPublisher_RoleCreatedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startRoleJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectRoleCreated)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const spaceID = "11111111-1111-1111-1111-111111111111"
	const roleID = "22222222-2222-2222-2222-222222222222"
	require.NoError(t, pub.PublishRoleCreated(ctx, spaceID, roleID, "Member"))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	require.NotEmpty(t, msg.Data)
}

// TestJetStreamPublisher_RoleAssignedRoundTrip documents role.assigned payload fields.
func TestJetStreamPublisher_RoleAssignedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startRoleJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectRoleAssigned)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	require.NoError(t, pub.PublishRoleAssigned(ctx,
		"33333333-3333-3333-3333-333333333333",
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
	))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	require.NotEmpty(t, msg.Data)
}

// TestJetStreamPublisher_ChatOverrideSetRoundTrip documents role.chat_override_set event.
func TestJetStreamPublisher_ChatOverrideSetRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startRoleJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectChatOverride)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	require.NoError(t, pub.PublishChatOverrideSet(ctx,
		"cccccccc-cccc-cccc-cccc-cccccccccccc",
		"dddddddd-dddd-dddd-dddd-dddddddddddd",
	))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	require.NotEmpty(t, msg.Data)
}
