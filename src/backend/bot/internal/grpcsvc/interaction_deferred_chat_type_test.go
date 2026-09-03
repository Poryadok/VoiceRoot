package grpcsvc_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestExecuteSlashInteraction_persistsChatTypeForDeferredRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	for _, tc := range []struct {
		name     string
		chatType chatv1.ChatType
	}{
		{name: "DM", chatType: chatv1.ChatType_CHAT_TYPE_DM},
		{name: "group", chatType: chatv1.ChatType_CHAT_TYPE_GROUP},
		{name: "channel", chatType: chatv1.ChatType_CHAT_TYPE_CHANNEL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &fakeMessagingClient{}
			client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
			defer cleanup()

			ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
			botUUID, err := uuid.Parse(botID)
			require.NoError(t, err)

			responseCh := make(chan *botv1.ExecuteSlashInteractionResponse, 1)
			errCh := make(chan error, 1)
			go func() {
				resp, callErr := client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
					Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &tc.chatType},
					BotId:       botID,
					CommandName: "ping",
				})
				if callErr != nil {
					errCh <- callErr
					return
				}
				responseCh <- resp
			}()

			token, payload := waitForPersistedInteraction(t, st, botUUID)
			require.Equal(t, tc.chatType.String(), payload["chat_type"])

			_, err = client.CompleteInteraction(withBotToken(context.Background(), botToken), &botv1.CompleteInteractionRequest{
				InteractionToken: token,
				Content:          "pong",
			})
			require.NoError(t, err)

			select {
			case callErr := <-errCh:
				require.NoError(t, callErr)
			case resp := <-responseCh:
				require.Equal(t, "pong", resp.GetMessage().GetContent())
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for slash interaction")
			}
		})
	}
}

func TestDeferredInteraction_recoveryRestoresPersistedChatType(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	for _, tc := range []struct {
		name     string
		chatType chatv1.ChatType
		deferred bool
		followUp func(botv1.BotServiceClient, context.Context, string) error
	}{
		{
			name:     "CompleteInteraction DM",
			chatType: chatv1.ChatType_CHAT_TYPE_DM,
			deferred: false,
			followUp: func(client botv1.BotServiceClient, ctx context.Context, token string) error {
				_, err := client.CompleteInteraction(ctx, &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "recovered"})
				return err
			},
		},
		{
			name:     "CompleteInteraction group",
			chatType: chatv1.ChatType_CHAT_TYPE_GROUP,
			deferred: false,
			followUp: func(client botv1.BotServiceClient, ctx context.Context, token string) error {
				_, err := client.CompleteInteraction(ctx, &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "recovered"})
				return err
			},
		},
		{
			name:     "CompleteInteraction channel",
			chatType: chatv1.ChatType_CHAT_TYPE_CHANNEL,
			deferred: false,
			followUp: func(client botv1.BotServiceClient, ctx context.Context, token string) error {
				_, err := client.CompleteInteraction(ctx, &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "recovered"})
				return err
			},
		},
		{
			name:     "SendBotMessage deferred DM",
			chatType: chatv1.ChatType_CHAT_TYPE_DM,
			deferred: true,
			followUp: func(client botv1.BotServiceClient, ctx context.Context, token string) error {
				_, err := client.SendBotMessage(ctx, &botv1.SendBotMessageRequest{InteractionToken: &token, Content: "recovered"})
				return err
			},
		},
		{
			name:     "SendBotMessage deferred group",
			chatType: chatv1.ChatType_CHAT_TYPE_GROUP,
			deferred: true,
			followUp: func(client botv1.BotServiceClient, ctx context.Context, token string) error {
				_, err := client.SendBotMessage(ctx, &botv1.SendBotMessageRequest{InteractionToken: &token, Content: "recovered"})
				return err
			},
		},
		{
			name:     "SendBotMessage deferred channel",
			chatType: chatv1.ChatType_CHAT_TYPE_CHANNEL,
			deferred: true,
			followUp: func(client botv1.BotServiceClient, ctx context.Context, token string) error {
				_, err := client.SendBotMessage(ctx, &botv1.SendBotMessageRequest{InteractionToken: &token, Content: "recovered"})
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &fakeMessagingClient{}
			client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
			defer cleanup()

			ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
			botUUID, err := uuid.Parse(botID)
			require.NoError(t, err)
			token := "recover-chat-type-" + uuid.NewString()
			_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
				"chat_id":            chatID.String(),
				"chat_type":          tc.chatType.String(),
				"invoker_profile_id": uuid.NewString(),
			}, token)
			require.NoError(t, err)
			require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
			if tc.deferred {
				hub.RegisterDeferred(token)
			}

			require.NoError(t, tc.followUp(client, withBotToken(context.Background(), botToken), token))
			require.NotNil(t, msg.lastChat)
			require.Equal(t, tc.chatType, msg.lastChat.GetType())
		})
	}
}

func TestDeferredInteraction_legacyOrInvalidChatTypeFallsBackToChannel(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	for _, tc := range []struct {
		name    string
		payload map[string]any
	}{
		{name: "legacy missing chat_type", payload: map[string]any{}},
		{name: "unknown enum name", payload: map[string]any{"chat_type": "CHAT_TYPE_UNKNOWN"}},
		{name: "malformed value", payload: map[string]any{"chat_type": 99}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &fakeMessagingClient{}
			client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
			defer cleanup()

			ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
			botUUID, err := uuid.Parse(botID)
			require.NoError(t, err)
			token := "recover-legacy-chat-type-" + uuid.NewString()
			payload := map[string]any{
				"chat_id":            chatID.String(),
				"invoker_profile_id": uuid.NewString(),
			}
			for key, value := range tc.payload {
				payload[key] = value
			}
			_, err = st.EnqueueEvent(ctx, botUUID, "interaction", payload, token)
			require.NoError(t, err)
			require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))

			_, err = client.CompleteInteraction(withBotToken(context.Background(), botToken), &botv1.CompleteInteractionRequest{
				InteractionToken: token,
				Content:          "recovered",
			})
			require.NoError(t, err)
			require.NotNil(t, msg.lastChat)
			require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, msg.lastChat.GetType())
		})
	}
}

func waitForPersistedInteraction(t *testing.T, st interface {
	ListPendingEvents(context.Context, uuid.UUID, int) ([]uuid.UUID, []string, []string, error)
}, botID uuid.UUID) (string, map[string]any) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, _, payloads, err := st.ListPendingEvents(context.Background(), botID, 5)
		if err == nil && len(payloads) > 0 {
			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(payloads[0]), &payload))
			token, _ := payload["interaction_token"].(string)
			if token != "" {
				return token, payload
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout waiting for persisted interaction")
	return "", nil
}
