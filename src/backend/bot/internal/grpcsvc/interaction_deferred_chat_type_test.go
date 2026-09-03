package grpcsvc_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// strictDestinationMessagingClient makes a recovery test fail at the Messaging
// boundary if the Bot service substitutes a different persisted destination.
type strictDestinationMessagingClient struct {
	messagingv1.UnimplementedMessagingServiceServer
	mu               sync.Mutex
	expectedChatID   string
	expectedChatType chatv1.ChatType
	sendCalls        int
	lastChat         *chatv1.ChatRef
	sendErr          error
}

func (f *strictDestinationMessagingClient) SendMessage(_ context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendCalls++
	f.lastChat = req.GetChat()
	if f.lastChat == nil || f.lastChat.GetId() != f.expectedChatID || f.lastChat.GetType() != f.expectedChatType {
		return nil, status.Errorf(codes.InvalidArgument, "unexpected interaction destination: got id=%q type=%s, want id=%q type=%s",
			f.lastChat.GetId(), f.lastChat.GetType(), f.expectedChatID, f.expectedChatType)
	}
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return &messagingv1.SendMessageResponse{
		Message: &messagingv1.Message{Id: uuid.NewString(), Content: req.GetContent()},
	}, nil
}

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

			scopesJSON := `["TEXT_CHAT_SEND_MESSAGES"]`
			if tc.chatType == chatv1.ChatType_CHAT_TYPE_DM {
				scopesJSON = `["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`
			}
			ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, scopesJSON)
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
			msg := &strictDestinationMessagingClient{expectedChatType: tc.chatType}
			client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
			defer cleanup()

			scopesJSON := `["TEXT_CHAT_SEND_MESSAGES"]`
			if tc.chatType == chatv1.ChatType_CHAT_TYPE_DM {
				scopesJSON = `["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`
			}
			ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, scopesJSON)
			msg.expectedChatID = chatID.String()
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
			require.Equal(t, 1, msg.sendCalls)
			require.NotNil(t, msg.lastChat)
			require.Equal(t, chatID.String(), msg.lastChat.GetId())
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
		{name: "real unspecified enum name", payload: map[string]any{"chat_type": "CHAT_TYPE_UNSPECIFIED"}},
		{name: "numeric unspecified enum zero", payload: map[string]any{"chat_type": 0}},
		{name: "unknown enum name", payload: map[string]any{"chat_type": "CHAT_TYPE_UNKNOWN"}},
		{name: "malformed value", payload: map[string]any{"chat_type": true}},
		{name: "unknown numeric enum value", payload: map[string]any{"chat_type": 99}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &strictDestinationMessagingClient{expectedChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL}
			client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
			defer cleanup()

			ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
			msg.expectedChatID = chatID.String()
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
			require.Equal(t, 1, msg.sendCalls)
			require.NotNil(t, msg.lastChat)
			require.Equal(t, chatID.String(), msg.lastChat.GetId())
			require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, msg.lastChat.GetType())
		})
	}
}

func TestDeferredDMInteraction_requiresScopeAndOriginalInteraction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Run("authorized original interaction succeeds", func(t *testing.T) {
		msg := &strictDestinationMessagingClient{expectedChatType: chatv1.ChatType_CHAT_TYPE_DM}
		client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
		defer cleanup()

		ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`)
		msg.expectedChatID = chatID.String()
		botUUID, err := uuid.Parse(botID)
		require.NoError(t, err)
		token := "authorized-dm-" + uuid.NewString()
		_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
			"chat_id":            chatID.String(),
			"chat_type":          chatv1.ChatType_CHAT_TYPE_DM.String(),
			"invoker_profile_id": uuid.NewString(),
		}, token)
		require.NoError(t, err)
		require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
		hub.RegisterDeferred(token)

		_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{
			InteractionToken: &token,
			Content:          "reply to original DM",
		})
		require.NoError(t, err)
		require.Equal(t, 1, msg.sendCalls)
	})

	t.Run("explicit original DM and token succeeds", func(t *testing.T) {
		dmType := chatv1.ChatType_CHAT_TYPE_DM
		msg := &strictDestinationMessagingClient{expectedChatType: dmType}
		client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
		defer cleanup()

		ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`)
		msg.expectedChatID = chatID.String()
		botUUID, err := uuid.Parse(botID)
		require.NoError(t, err)
		token := "explicit-original-dm-" + uuid.NewString()
		_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
			"chat_id":            chatID.String(),
			"chat_type":          dmType.String(),
			"invoker_profile_id": uuid.NewString(),
		}, token)
		require.NoError(t, err)
		require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
		hub.RegisterDeferred(token)

		_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{
			Chat:             &chatv1.ChatRef{Id: chatID.String(), Type: &dmType},
			InteractionToken: &token,
			Content:          "reply to original DM",
		})
		require.NoError(t, err)
		require.Equal(t, 1, msg.sendCalls)
		require.NotNil(t, msg.lastChat)
		require.Equal(t, chatID.String(), msg.lastChat.GetId())
		require.Equal(t, dmType, msg.lastChat.GetType())
	})

	t.Run("missing DM_SEND scope is denied even for original interaction", func(t *testing.T) {
		msg := &strictDestinationMessagingClient{expectedChatType: chatv1.ChatType_CHAT_TYPE_DM}
		client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
		defer cleanup()

		ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
		msg.expectedChatID = chatID.String()
		botUUID, err := uuid.Parse(botID)
		require.NoError(t, err)
		token := "missing-dm-send-" + uuid.NewString()
		_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
			"chat_id":            chatID.String(),
			"chat_type":          chatv1.ChatType_CHAT_TYPE_DM.String(),
			"invoker_profile_id": uuid.NewString(),
		}, token)
		require.NoError(t, err)
		require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))

		_, err = client.CompleteInteraction(withBotToken(context.Background(), botToken), &botv1.CompleteInteractionRequest{
			InteractionToken: token,
			Content:          "must not send",
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.Contains(t, status.Convert(err).Message(), "DM_SEND")
		require.Zero(t, msg.sendCalls)
	})

	t.Run("missing DM_SEND scope is denied for deferred Hub completion", func(t *testing.T) {
		msg := &strictDestinationMessagingClient{expectedChatType: chatv1.ChatType_CHAT_TYPE_DM}
		client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
		defer cleanup()

		ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
		botUUID, err := uuid.Parse(botID)
		require.NoError(t, err)
		token := "missing-dm-send-deferred-" + uuid.NewString()
		_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
			"chat_id":            chatID.String(),
			"chat_type":          chatv1.ChatType_CHAT_TYPE_DM.String(),
			"invoker_profile_id": uuid.NewString(),
		}, token)
		require.NoError(t, err)
		require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
		hub.RegisterDeferred(token)

		_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{
			InteractionToken: &token,
			Content:          "must not send",
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.Contains(t, status.Convert(err).Message(), "DM_SEND")
		require.Zero(t, msg.sendCalls)
	})

	t.Run("persisted deferred token cannot authorize another whitelisted DM", func(t *testing.T) {
		dmType := chatv1.ChatType_CHAT_TYPE_DM
		msg := &strictDestinationMessagingClient{expectedChatType: dmType}
		client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
		defer cleanup()

		ctx, botID, botToken, originalDM, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`)
		botUUID, err := uuid.Parse(botID)
		require.NoError(t, err)
		profile, _ := authProfile(ctx)

		destinationDM := uuid.New()
		require.NoError(t, st.SetChatEnabled(ctx, botUUID, destinationDM, spaceID, profile, true))
		msg.expectedChatID = originalDM.String()

		token := "deferred-dm-a-" + uuid.NewString()
		_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
			"chat_id":            originalDM.String(),
			"chat_type":          dmType.String(),
			"invoker_profile_id": uuid.NewString(),
		}, token)
		require.NoError(t, err)
		require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))

		_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{
			Chat:             &chatv1.ChatRef{Id: destinationDM.String(), Type: &dmType},
			InteractionToken: &token,
			Content:          "must not send",
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.Zero(t, msg.sendCalls)
	})

	t.Run("deferred token is single use under concurrent sends", func(t *testing.T) {
		channelType := chatv1.ChatType_CHAT_TYPE_CHANNEL
		msg := &strictDestinationMessagingClient{expectedChatType: channelType}
		client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
		defer cleanup()

		ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
		msg.expectedChatID = chatID.String()
		botUUID, err := uuid.Parse(botID)
		require.NoError(t, err)
		token := "single-use-deferred-" + uuid.NewString()
		_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
			"chat_id":            chatID.String(),
			"chat_type":          channelType.String(),
			"invoker_profile_id": uuid.NewString(),
		}, token)
		require.NoError(t, err)
		require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
		hub.RegisterDeferred(token)

		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{
					InteractionToken: &token,
					Content:          "exactly once",
				})
				errs <- err
			}()
		}
		close(start)
		wg.Wait()
		close(errs)

		var success, rejected int
		for err := range errs {
			if err == nil {
				success++
				continue
			}
			require.Equal(t, codes.NotFound, status.Code(err))
			rejected++
		}
		require.Equal(t, 1, success)
		require.Equal(t, 1, rejected)
		require.Equal(t, 1, msg.sendCalls)
	})

	t.Run("empty deferred content does not consume token", func(t *testing.T) {
		channelType := chatv1.ChatType_CHAT_TYPE_CHANNEL
		msg := &strictDestinationMessagingClient{expectedChatType: channelType}
		client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
		defer cleanup()

		ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
		msg.expectedChatID = chatID.String()
		botUUID, err := uuid.Parse(botID)
		require.NoError(t, err)
		token := "empty-retry-deferred-" + uuid.NewString()
		_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
			"chat_id": chatID.String(), "chat_type": channelType.String(), "invoker_profile_id": uuid.NewString(),
		}, token)
		require.NoError(t, err)
		require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
		hub.RegisterDeferred(token)

		_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{InteractionToken: &token, Content: " "})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Zero(t, msg.sendCalls)
		_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{InteractionToken: &token, Content: "retry"})
		require.NoError(t, err)
		require.Equal(t, 1, msg.sendCalls)
	})

	t.Run("Messaging failure releases deferred token for retry", func(t *testing.T) {
		channelType := chatv1.ChatType_CHAT_TYPE_CHANNEL
		msg := &strictDestinationMessagingClient{expectedChatType: channelType, sendErr: status.Error(codes.Unavailable, "temporary")}
		client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
		defer cleanup()

		ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
		msg.expectedChatID = chatID.String()
		botUUID, err := uuid.Parse(botID)
		require.NoError(t, err)
		token := "failure-retry-deferred-" + uuid.NewString()
		_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
			"chat_id": chatID.String(), "chat_type": channelType.String(), "invoker_profile_id": uuid.NewString(),
		}, token)
		require.NoError(t, err)
		require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
		hub.RegisterDeferred(token)

		_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{InteractionToken: &token, Content: "retry"})
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Equal(t, 1, msg.sendCalls)
		msg.mu.Lock()
		msg.sendErr = nil
		msg.mu.Unlock()
		_, err = client.SendBotMessage(withBotToken(context.Background(), botToken), &botv1.SendBotMessageRequest{InteractionToken: &token, Content: "retry"})
		require.NoError(t, err)
		require.Equal(t, 2, msg.sendCalls)
	})
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
