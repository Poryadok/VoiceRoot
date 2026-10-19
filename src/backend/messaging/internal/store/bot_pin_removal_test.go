package store

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/messageevents"
)

func botPinRemovalRequest(chat, space, actor uuid.UUID) *messagingv1.ApplyBotChatPinRemovalRequest {
	return &messagingv1.ApplyBotChatPinRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: uuid.NewString(), DecisionSha256: []byte("01234567890123456789012345678901"), Purpose: "global_delete", BotId: uuid.NewString(), ActorProfileId: actor.String(), SpaceId: space.String(), ChatId: chat.String(), FrozenScopeSha256: []byte("abcdefghijklmnopqrstuvwxyz012345"), Evidence: &messagingv1.ApplyBotChatPinRemovalRequest_Native{Native: &messagingv1.BotRemovalNativeEvidence{InstallationId: uuid.NewString(), InstallOperationId: uuid.NewString(), Generation: 1}}}
}

func TestBotPinCleanupRejectsStalePinAndReplayPreservesFreshRepin(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL cleanup/pin serialization")
	}
	ctx := t.Context()
	pool := startPostgresForStoreTest(t, ctx)
	t.Cleanup(pool.Close)
	seedMessagingSchema(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000035_bot_pin_removal.up.sql"))
	pins := &PinsStore{Pool: pool}
	chat, space, actor, human, foreignActor := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	message, foreignMessage, foreignChat, otherMessage := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedMessage(t, ctx, pool, chat, message, actor)
	seedMessage(t, ctx, pool, chat, foreignMessage, foreignActor)
	seedMessage(t, ctx, pool, foreignChat, otherMessage, actor)
	for _, row := range []struct{ chat, message uuid.UUID }{{chat, message}, {chat, foreignMessage}, {foreignChat, otherMessage}} {
		require.NoError(t, pins.UpsertPin(ctx, row.chat, row.message, human))
	}
	admission, err := pins.ObservePinCleanupRevision(ctx, chat, message)
	require.NoError(t, err)
	request := botPinRemovalRequest(chat, space, actor)
	response, err := pins.ApplyBotChatPinRemoval(ctx, request)
	require.NoError(t, err)
	require.EqualValues(t, 1, response.GetReceipt().GetRemovedPinCount())
	staleEvent, err := messageevents.NewMessagePinnedOutbox(message.String(), chat.String(), human.String())
	require.NoError(t, err)
	err = pins.MutatePinWithOutbox(WithPinCleanupObservation(ctx, admission), chat, space, message, human, true, staleEvent)
	if err == nil {
		var pinCount, eventCount int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pins WHERE chat_id=$1 AND message_id=$2`, chat, message).Scan(&pinCount))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE event_id=$1`, staleEvent.EventID).Scan(&eventCount))
		t.Logf("unfenced stale attempt committed pin=%d outbox=%d", pinCount, eventCount)
	}
	require.ErrorIs(t, err, ErrPinCleanupStale, "pre-cleanup pin must not recreate a pin or emit outbox")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE event_id=$1`, staleEvent.EventID).Scan(&count))
	require.Zero(t, count)
	remaining, err := pins.ListPins(ctx, chat)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	require.Equal(t, foreignMessage, remaining[0].MessageID)
	otherPins, err := pins.ListPins(ctx, foreignChat)
	require.NoError(t, err)
	require.Len(t, otherPins, 1)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE id=ANY($1)`, []uuid.UUID{message, foreignMessage, otherMessage}).Scan(&count))
	require.Equal(t, 3, count, "cleanup preserves messages and foreign scope")
	fresh, err := pins.ObservePinCleanupRevision(ctx, chat, message)
	require.NoError(t, err)
	freshEvent, err := messageevents.NewMessagePinnedOutbox(message.String(), chat.String(), human.String())
	require.NoError(t, err)
	require.NoError(t, pins.MutatePinWithOutbox(WithPinCleanupObservation(ctx, fresh), chat, space, message, human, true, freshEvent), "fresh authorized human repin is not permanently blocked")
	restarted := &PinsStore{Pool: pool}
	replay, err := restarted.ApplyBotChatPinRemoval(ctx, proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(response, replay), "same exact cleanup returns immutable receipt")
	remaining, err = pins.ListPins(ctx, chat)
	require.NoError(t, err)
	require.Len(t, remaining, 2, "replay does not remove a later fresh repin")
	changed := proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest)
	changed.ChatId = foreignChat.String()
	_, err = restarted.ApplyBotChatPinRemoval(ctx, changed)
	require.ErrorIs(t, err, ErrBotPinRemovalConflict)
}
