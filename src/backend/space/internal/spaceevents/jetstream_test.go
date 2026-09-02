package spaceevents

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func TestNewJetStreamPublisher_EmptyURL(t *testing.T) {
	_, err := NewJetStreamPublisher("")
	require.Error(t, err)
}

func TestStreamHasSubject(t *testing.T) {
	t.Parallel()
	require.False(t, streamHasSubject(nil, subjectSpaceCreated))
	info := &nats.StreamInfo{Config: nats.StreamConfig{Subjects: []string{"chat.created"}}}
	require.False(t, streamHasSubject(info, subjectSpaceCreated))
	info.Config.Subjects = append(info.Config.Subjects, subjectSpaceCreated)
	require.True(t, streamHasSubject(info, subjectSpaceCreated))
}

func TestJetStreamPublisher_CloseNil(t *testing.T) {
	var p *JetStreamPublisher
	require.NoError(t, p.Close())
}

func startJSTestServer(t *testing.T) *server.Server {
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

// TestJetStreamPublisher_SpaceCreatedRoundTrip documents space-service.md:
// space.created on chat.events stream with space_id and owner_profile_id.
func TestJetStreamPublisher_SpaceCreatedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectSpaceCreated)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const spaceID = "11111111-1111-1111-1111-111111111111"
	const ownerID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	require.NoError(t, pub.PublishSpaceCreated(ctx, spaceID, ownerID))

	msg, err := sub.NextMsg(10 * time.Second)
	require.NoError(t, err)
	var env eventsv1.ChatStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	require.NotEmpty(t, env.GetEventId())
	require.NotNil(t, env.GetOccurredAt())
	created := env.GetSpaceCreated()
	require.NotNil(t, created)
	require.Equal(t, spaceID, created.GetSpaceId())
	require.Equal(t, ownerID, created.GetOwnerProfileId())
}

func TestJetStreamPublisher_InviteCreatedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectSpaceInviteCreated)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const spaceID = "33333333-3333-3333-3333-333333333333"
	const inviteCode = "invite-code-abc"
	require.NoError(t, pub.PublishInviteCreated(ctx, spaceID, inviteCode))

	msg, err := sub.NextMsg(10 * time.Second)
	require.NoError(t, err)
	var env eventsv1.ChatStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	require.NotEmpty(t, env.GetEventId())
	require.NotNil(t, env.GetOccurredAt())
	invite := env.GetSpaceInviteCreated()
	require.NotNil(t, invite, "space.invite_created must use SpaceInviteCreated payload, not SpaceCreated")
	require.Equal(t, spaceID, invite.GetSpaceId())
	require.Equal(t, inviteCode, invite.GetInviteCode())
	require.Nil(t, env.GetSpaceCreated(), "invite_code must not be written into owner_profile_id")
}

func TestJetStreamPublisher_TreeNodeUpsertedIncludesPinFields(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectSpaceTreeChanged)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	pinOrder := int32(2)
	require.NoError(t, pub.PublishTreeNodeUpserted(ctx,
		"space-1", "node-1", "text_chat", "chat-1", "", true, &pinOrder))

	msg, err := sub.NextMsg(10 * time.Second)
	require.NoError(t, err)
	var env eventsv1.ChatStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	changed := env.GetSpaceTreeChanged()
	require.NotNil(t, changed)
	require.Equal(t, "upserted", changed.GetChange())
	require.Equal(t, "text_chat", changed.GetKind())
	require.Equal(t, "chat-1", changed.GetChatId())
	require.True(t, changed.GetIsPinned())
	require.Equal(t, int32(2), changed.GetPinOrder())
}

// TestJetStreamPublisher_EnsureStreamUpdatesExisting documents stream subject migration when chat_events exists without space.created.
func TestJetStreamPublisher_EnsureStreamUpdatesExisting(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{"chat.created"},
		Retention: nats.LimitsPolicy,
	})
	require.NoError(t, err)

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	require.NoError(t, pub.PublishSpaceCreated(ctx, "22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"))

	info, err := js.StreamInfo(streamName)
	require.NoError(t, err)
	require.True(t, streamHasSubject(info, subjectSpaceCreated))
}
