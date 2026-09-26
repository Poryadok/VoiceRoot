package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestSendBotMessageRequiresSendScopeOnOrdinaryPath(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()
	_, botID, token, chatID, _ := setupBotCCommandBot(t, client, st, `["DM_SEND"]`)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err := client.SendBotMessage(withBotToken(context.Background(), token), &botv1.SendBotMessageRequest{
		BotId: botID, Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}, Content: "denied",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Empty(t, msg.lastContent)
}

func TestSendBotMessagePropagatesThreadParent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()
	_, botID, token, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	parent := uuid.NewString()
	_, err := client.SendBotMessage(withBotToken(context.Background(), token), &botv1.SendBotMessageRequest{
		BotId: botID, Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}, Content: "reply", ThreadParentId: &parent,
	})
	require.NoError(t, err)
	require.Equal(t, parent, msg.lastThreadParent)
}

func TestDeferredBotMessagePropagatesThreadParent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()
	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	token, parent := uuid.NewString(), uuid.NewString()
	_, err := st.EnqueueEvent(ctx, uuid.MustParse(botID), "interaction", map[string]any{
		"chat_id": chatID.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString(),
	}, token)
	require.NoError(t, err)
	require.NoError(t, st.MarkEventDeferred(ctx, uuid.MustParse(botID), token))
	_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{
		InteractionToken: &token, ThreadParentId: &parent, Content: "threaded reply",
	})
	require.NoError(t, err)
	require.Equal(t, parent, msg.lastThreadParent)
}

func TestSlashDiscoveryAndAutocompleteRequireChatMembership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chat := &fakeChatClient{listErr: status.Error(codes.PermissionDenied, "removed member")}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{chat: chat})
	defer cleanup()
	ctx, botID, _, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	ref := &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}
	_, err := client.ListSlashCommandsForChat(ctx, &botv1.ListSlashCommandsForChatRequest{Chat: ref})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = client.AutocompleteSlashOption(ctx, &botv1.AutocompleteSlashOptionRequest{
		BotId: botID, Chat: ref, CommandName: "ping", OptionName: "query", FocusedValue: "a",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
