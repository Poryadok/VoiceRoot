package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
)

// This exercises real Messaging transactions/outbox, with an explicit Space
// decision seam. It is not deployment or active-media acceptance evidence.
func TestCommunicationTimeoutBlocksEveryEnabledOrdinaryEffectAndAllowsExactReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL Messaging effects")
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	chat, space, profile, peer, account := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, chat, profile, peer)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id=$2,threads_enabled=true WHERE id=$1`, chat, space)
	require.NoError(t, err)
	authority := &communicationStateStub{state: 1}
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{CommunicationStates: authority, RolePermissions: testRolePerms{allowed: true}})
	defer cleanup()
	caller := withProfileCtx(ctx, account, profile)
	request := &messagingv1.SendMessageRequest{Chat: chatGroupRef(chat), Content: "persisted exact receipt", ClientMessageId: proto.String(uuid.NewString())}
	sent, err := client.SendMessage(caller, request)
	require.NoError(t, err)
	message := sent.GetMessage().GetId()
	_, err = client.AddReaction(caller, &messagingv1.AddReactionRequest{MessageId: message, Emoji: "👍"})
	require.NoError(t, err)
	_, err = client.PinMessage(caller, &messagingv1.PinMessageRequest{Chat: chatGroupRef(chat), MessageId: message})
	require.NoError(t, err)
	counts := func() []int {
		var messages, events, reactions, pins, intents int
		require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM message_event_outbox),(SELECT count(*) FROM reactions),(SELECT count(*) FROM pins),(SELECT count(*) FROM messaging_attachment_send_intents)`).Scan(&messages, &events, &reactions, &pins, &intents))
		return []int{messages, events, reactions, pins, intents}
	}
	before := counts()
	authority.state = 2
	for _, action := range []struct {
		name string
		run  func() error
	}{
		{"send", func() error {
			_, err := client.SendMessage(caller, &messagingv1.SendMessageRequest{Chat: chatDMRef(chat), Content: "forged type cannot bypass stored Space"})
			return err
		}},
		{"attachment", func() error {
			_, err := client.SendMessage(caller, &messagingv1.SendMessageRequest{Chat: chatGroupRef(chat), AttachmentsJson: `[{"file_id":"` + uuid.NewString() + `","type":"photo"}]`})
			return err
		}},
		{"reply", func() error {
			_, err := client.SendMessage(caller, &messagingv1.SendMessageRequest{Chat: chatGroupRef(chat), Content: "reply", ThreadParentId: proto.String(message)})
			return err
		}},
		{"forward and commentary", func() error {
			comment := "comment"
			_, err := client.ForwardMessage(caller, &messagingv1.ForwardMessageRequest{SourceMessageId: message, TargetChat: chatGroupRef(chat), Commentary: &comment})
			return err
		}},
		{"edit", func() error {
			_, err := client.EditMessage(caller, &messagingv1.EditMessageRequest{MessageId: message, Content: "changed"})
			return err
		}},
		{"reaction add", func() error {
			_, err := client.AddReaction(caller, &messagingv1.AddReactionRequest{MessageId: message, Emoji: "🔥"})
			return err
		}},
		{"pin", func() error {
			_, err := client.PinMessage(caller, &messagingv1.PinMessageRequest{Chat: chatGroupRef(chat), MessageId: message})
			return err
		}},
		{"unpin", func() error {
			_, err := client.UnpinMessage(caller, &messagingv1.UnpinMessageRequest{Chat: chatGroupRef(chat), MessageId: message})
			return err
		}},
	} {
		t.Run(action.name, func(t *testing.T) {
			require.Equal(t, codes.PermissionDenied, status.Code(action.run()))
			require.Equal(t, before, counts(), "denial must leave all durable effects unchanged")
		})
	}
	lookups := authority.calls
	replayed, err := client.SendMessage(caller, request)
	require.NoError(t, err)
	require.Equal(t, message, replayed.GetMessage().GetId())
	require.Equal(t, lookups, authority.calls, "exact receipt creates no communication effect and needs no timeout decision")
	require.Equal(t, before, counts())
	changed := proto.Clone(request).(*messagingv1.SendMessageRequest)
	changed.Content = "same key is not the same request"
	_, err = client.SendMessage(caller, changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.Equal(t, before, counts())
	_, err = client.RemoveReaction(caller, &messagingv1.RemoveReactionRequest{MessageId: message, Emoji: "👍"})
	require.NoError(t, err, "own reaction removal is cleanup, not a denied communication effect")
}
