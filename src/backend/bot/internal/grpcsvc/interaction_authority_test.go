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

func TestCompleteInteractionCannotConsumeAnotherBotsLiveToken(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()
	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	other, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{Name: "OtherBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`})
	require.NoError(t, err)
	token := uuid.NewString()
	_, err = st.EnqueueEvent(ctx, uuid.MustParse(botID), "interaction", map[string]any{
		"chat_id": chatID.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString(),
	}, token)
	require.NoError(t, err)
	hub.Register(token)
	_, err = client.CompleteInteraction(withBotToken(context.Background(), other.GetTokenResponse().GetToken()), &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "spoof"})
	require.Equal(t, codes.NotFound, status.Code(err))
	require.True(t, hub.IsPending(token))
	_, err = client.CompleteInteraction(withBotToken(context.Background(), botToken), &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "valid"})
	require.NoError(t, err)
}

func TestSendBotMessageCannotCompleteLiveTokenForDifferentChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()
	ctx, botID, botToken, chatID, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID := uuid.MustParse(botID)
	otherChat := uuid.New()
	require.NoError(t, st.SetChatEnabled(ctx, botUUID, otherChat, spaceID, uuid.New(), true))
	token := uuid.NewString()
	_, err := st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
		"chat_id": chatID.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString(),
	}, token)
	require.NoError(t, err)
	hub.Register(token)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{
		InteractionToken: &token, Chat: &chatv1.ChatRef{Id: otherChat.String(), Type: &chatType}, Content: "spoof",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.True(t, hub.IsPending(token))
}

func TestCompleteDeferredInteractionPostsDespiteRestoredHubToken(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()
	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	token := uuid.NewString()
	_, err := st.EnqueueEvent(ctx, uuid.MustParse(botID), "interaction", map[string]any{
		"chat_id": chatID.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString(),
	}, token)
	require.NoError(t, err)
	require.NoError(t, st.MarkEventDeferred(ctx, uuid.MustParse(botID), token))
	hub.RegisterDeferred(token)
	_, err = client.CompleteInteraction(withBotToken(context.Background(), botToken), &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "done"})
	require.NoError(t, err)
	require.Equal(t, "done", msg.lastContent)
	require.False(t, hub.IsPending(token))
}
