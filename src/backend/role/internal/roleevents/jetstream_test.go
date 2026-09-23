package roleevents

import (
	"context"
	"encoding/json"
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

func provisionRoleEventStream(t *testing.T, url string) {
	t.Helper()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: []string{
		subjectRoleCreated, subjectRoleUpdated, subjectRoleDeleted, subjectRoleAssigned, subjectRoleRevoked,
		subjectChatOverride, subjectChatOverrideRemoved, subjectVoiceOverride, subjectVoiceOverrideRemoved,
	}})
	require.NoError(t, err)
}

func TestJetStreamPublisher_OverrideRemovalRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startRoleJSTestServer(t)
	provisionRoleEventStream(t, s.ClientURL())
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	chatSub := subscribeSync(t, nc, subjectChatOverrideRemoved)
	voiceSub := subscribeSync(t, nc, subjectVoiceOverrideRemoved)
	pub, err := NewJetStreamPublisher(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const spaceID = "11111111-1111-1111-1111-111111111111"
	const roleID = "22222222-2222-2222-2222-222222222222"
	const chatID = "33333333-3333-3333-3333-333333333333"
	const voiceRoomID = "44444444-4444-4444-4444-444444444444"
	require.NoError(t, pub.PublishChatOverrideRemoved(ctx, spaceID, chatID, roleID))
	require.NoError(t, pub.PublishVoiceOverrideRemoved(ctx, spaceID, voiceRoomID, roleID))

	chatMsg, err := chatSub.NextMsg(10 * time.Second)
	require.NoError(t, err)
	var chatPayload roleEventPayload
	require.NoError(t, json.Unmarshal(chatMsg.Data, &chatPayload))
	require.Equal(t, roleEventPayload{SpaceID: spaceID, ChatID: chatID, RoleID: roleID}, chatPayload)

	voiceMsg, err := voiceSub.NextMsg(10 * time.Second)
	require.NoError(t, err)
	var voicePayload roleEventPayload
	require.NoError(t, json.Unmarshal(voiceMsg.Data, &voicePayload))
	require.Equal(t, roleEventPayload{SpaceID: spaceID, VoiceRoomID: voiceRoomID, RoleID: roleID}, voicePayload)
}

func subscribeSync(t *testing.T, nc *nats.Conn, subject string) *nats.Subscription {
	t.Helper()
	sub, err := nc.SubscribeSync(subject)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	require.NoError(t, nc.Flush())
	return sub
}

func TestNewJetStreamPublisher_EmptyURL(t *testing.T) {
	_, err := NewJetStreamPublisher("")
	require.Error(t, err)
}

// TestJetStreamPublisher_RoleCreatedRoundTrip documents role-service.md role.created on role.events.
func TestJetStreamPublisher_RoleCreatedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startRoleJSTestServer(t)
	url := s.ClientURL()
	provisionRoleEventStream(t, url)

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub := subscribeSync(t, nc, subjectRoleCreated)

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const spaceID = "11111111-1111-1111-1111-111111111111"
	const roleID = "22222222-2222-2222-2222-222222222222"
	require.NoError(t, pub.PublishRoleCreated(ctx, spaceID, roleID, "Member"))

	msg, err := sub.NextMsg(10 * time.Second)
	require.NoError(t, err)
	require.NotEmpty(t, msg.Data)
}

// TestJetStreamPublisher_RoleAssignedRoundTrip documents role.assigned payload fields.
func TestJetStreamPublisher_RoleAssignedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startRoleJSTestServer(t)
	url := s.ClientURL()
	provisionRoleEventStream(t, url)

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub := subscribeSync(t, nc, subjectRoleAssigned)

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	require.NoError(t, pub.PublishRoleAssigned(ctx,
		"33333333-3333-3333-3333-333333333333",
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
	))

	msg, err := sub.NextMsg(10 * time.Second)
	require.NoError(t, err)
	require.NotEmpty(t, msg.Data)
}

// TestJetStreamPublisher_ChatOverrideSetRoundTrip documents role.chat_override_set event.
func TestJetStreamPublisher_ChatOverrideSetRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startRoleJSTestServer(t)
	url := s.ClientURL()
	provisionRoleEventStream(t, url)

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub := subscribeSync(t, nc, subjectChatOverride)

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	require.NoError(t, pub.PublishChatOverrideSet(ctx,
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		"cccccccc-cccc-cccc-cccc-cccccccccccc",
		"dddddddd-dddd-dddd-dddd-dddddddddddd",
	))

	msg, err := sub.NextMsg(10 * time.Second)
	require.NoError(t, err)
	require.NotEmpty(t, msg.Data)
}
