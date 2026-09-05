package chatevents

// RED contract for the Chat DM deletion publisher. The generated tag-19
// ChatStreamEvent API is the contract; unknown wire data is still rejected so
// this test proves the payload carries only the surviving recipient fields.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func requireDMPeerDeletedWireForTest(t *testing.T, data []byte, eventID string, chatID, recipientProfileID uuid.UUID) {
	t.Helper()
	var env eventsv1.ChatStreamEvent
	require.NoError(t, proto.Unmarshal(data, &env))
	require.Empty(t, env.ProtoReflect().GetUnknown(), "envelope must not contain unknown wire fields")
	require.Equal(t, eventID, env.GetEventId())
	require.NotNil(t, env.GetOccurredAt())
	payload := env.GetDmPeerDeleted()
	require.NotNil(t, payload)
	require.Empty(t, payload.ProtoReflect().GetUnknown(), "DM peer-deleted payload must not contain unknown wire fields")
	require.Equal(t, chatID.String(), payload.GetChatId())
	require.Equal(t, recipientProfileID.String(), payload.GetRecipientProfileId())
	require.True(t, proto.Equal(&eventsv1.ChatStreamEvent{
		EventId:    eventID,
		OccurredAt: env.GetOccurredAt(),
		Payload: &eventsv1.ChatStreamEvent_DmPeerDeleted{
			DmPeerDeleted: &eventsv1.DmPeerDeleted{
				ChatId:             chatID.String(),
				RecipientProfileId: recipientProfileID.String(),
			},
		},
	}, &env), "payload must be exactly chat_id and recipient_profile_id")
}

func TestJetStreamPublisher_DMPeerDeletedHasOnlySurvivorPayloadAndStableMsgID(t *testing.T) {
	server := startJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	const subject = "chat.dm_peer_deleted"
	sub, err := nc.SubscribeSync(subject)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	ctx := context.Background()
	chatID, survivorID, deletedID := uuid.New(), uuid.New(), uuid.New()
	const stableEventID = "f3c502d2-b56f-5eea-b132-27af463b00a6"
	require.NoError(t, pub.PublishDMPeerDeleted(ctx, stableEventID, chatID, survivorID))
	first, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	require.NoError(t, pub.PublishDMPeerDeleted(ctx, stableEventID, chatID, survivorID))
	second, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)

	requireDMPeerDeletedWireForTest(t, first.Data, stableEventID, chatID, survivorID)
	requireDMPeerDeletedWireForTest(t, second.Data, stableEventID, chatID, survivorID)
	require.Equal(t, stableEventID, first.Header.Get(nats.MsgIdHdr))
	require.Equal(t, stableEventID, second.Header.Get(nats.MsgIdHdr))
	require.NotContains(t, first.Data, []byte(deletedID.String()))
}

func TestJetStreamPublisher_ExistingChatStreamGainsDMPeerDeletedSubject(t *testing.T) {
	server := startJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{subjectChatCreated, subjectChatMemberChanged},
		Storage:  nats.MemoryStorage,
	})
	require.NoError(t, err)

	pub, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	require.NoError(t, pub.PublishDMPeerDeleted(context.Background(), uuid.NewString(), uuid.New(), uuid.New()))

	info, err := js.StreamInfo(streamName)
	require.NoError(t, err)
	require.Contains(t, info.Config.Subjects, subjectDMPeerDeleted)
}
