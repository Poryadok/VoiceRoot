package chatevents

// RED contract for the Chat DM deletion publisher.  The current worktree does
// not contain the accepted P0 tag-19 generated code, so the assertions decode
// the protobuf wire fields directly: ChatStreamEvent oneof field 19 contains
// only chat_id (1) and recipient_profile_id (2).

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
)

func callPublishDMPeerDeletedForTest(t *testing.T, pub *JetStreamPublisher, ctx context.Context, eventID, chatID, recipientProfileID string) {
	t.Helper()
	method := reflect.ValueOf(pub).MethodByName("PublishDMPeerDeleted")
	require.True(t, method.IsValid(), "JetStreamPublisher must expose PublishDMPeerDeleted")
	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(eventID),
		reflect.ValueOf(chatID),
		reflect.ValueOf(recipientProfileID),
	})
	require.Len(t, results, 1, "PublishDMPeerDeleted must return error")
	if !results[0].IsNil() {
		require.NoError(t, results[0].Interface().(error))
	}
}

func decodeDMPeerDeletedWireForTest(t *testing.T, data []byte) (eventID string, chatID string, recipientProfileID string, nestedFields []protowire.Number) {
	t.Helper()
	for len(data) > 0 {
		n, typ, tagLen := protowire.ConsumeTag(data)
		require.Greater(t, tagLen, 0)
		value, valueLen := protowire.ConsumeBytes(data[tagLen:])
		require.Greater(t, valueLen, 0)
		consumed := tagLen + valueLen
		data = data[consumed:]
		switch n {
		case 1:
			require.Equal(t, protowire.BytesType, typ)
			eventID = string(value)
		case 19:
			require.Equal(t, protowire.BytesType, typ)
			nested := value
			for len(nested) > 0 {
				field, fieldType, fieldTagLen := protowire.ConsumeTag(nested)
				require.Greater(t, fieldTagLen, 0)
				fieldValue, fieldValueLen := protowire.ConsumeBytes(nested[fieldTagLen:])
				require.Greater(t, fieldValueLen, 0)
				fieldConsumed := fieldTagLen + fieldValueLen
				nested = nested[fieldConsumed:]
				nestedFields = append(nestedFields, field)
				require.Equal(t, protowire.BytesType, fieldType)
				switch field {
				case 1:
					chatID = string(fieldValue)
				case 2:
					recipientProfileID = string(fieldValue)
				}
			}
		}
	}
	return eventID, chatID, recipientProfileID, nestedFields
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
	chatID, survivorID, deletedID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	const stableEventID = "dm-peer-delete-source-1-chat-1-survivor-1"
	callPublishDMPeerDeletedForTest(t, pub, ctx, stableEventID, chatID, survivorID)
	first, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	callPublishDMPeerDeletedForTest(t, pub, ctx, stableEventID, chatID, survivorID)
	second, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)

	firstEventID, firstChatID, firstRecipient, firstFields := decodeDMPeerDeletedWireForTest(t, first.Data)
	secondEventID, secondChatID, secondRecipient, secondFields := decodeDMPeerDeletedWireForTest(t, second.Data)
	require.Equal(t, stableEventID, firstEventID)
	require.Equal(t, stableEventID, secondEventID)
	require.Equal(t, firstEventID, secondEventID)
	require.Equal(t, first.Header.Get("Nats-Msg-Id"), second.Header.Get("Nats-Msg-Id"))
	require.NotEmpty(t, first.Header.Get("Nats-Msg-Id"))
	require.Equal(t, chatID, firstChatID)
	require.Equal(t, chatID, secondChatID)
	require.Equal(t, survivorID, firstRecipient)
	require.Equal(t, survivorID, secondRecipient)
	require.NotContains(t, first.Data, []byte(deletedID))
	require.ElementsMatch(t, []protowire.Number{1, 2}, firstFields)
	require.ElementsMatch(t, firstFields, secondFields)
}
