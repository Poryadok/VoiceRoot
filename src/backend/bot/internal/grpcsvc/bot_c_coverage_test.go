package grpcsvc_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	grpcsvc "voice/backend/bot/internal/grpcsvc"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
)

func TestGetChatMessagesForBot_deniedWithoutReadHistoryScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.GetChatMessagesForBot(botCtx, &botv1.GetChatMessagesForBotRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err),
		"GetChatMessagesForBot must require TEXT_CHAT_READ_HISTORY (BOT-C)")
}

func TestGetChatMessagesForBot_unimplementedWithPrivilegedScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES","TEXT_CHAT_READ_HISTORY"]`)
	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.GetChatMessagesForBot(botCtx, &botv1.GetChatMessagesForBotRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
	})
	require.Error(t, err)
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestGetChatMessagesForBot_returnsMessagesWithReadHistoryScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClientWithHistory{messageIDs: []string{"msg-a", "msg-b"}}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()

	_, _, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES","TEXT_CHAT_READ_HISTORY"]`)
	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	resp, err := client.GetChatMessagesForBot(botCtx, &botv1.GetChatMessagesForBotRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
	})
	require.NoError(t, err, "GetChatMessagesForBot must fetch history via Messaging (BOT-C)")
	require.Equal(t, []string{"msg-a", "msg-b"}, resp.GetMessageIds())
	require.Equal(t, 1, msg.getMessagesCalls)
}

func TestGetChatMessagesForBot_returnsMessageBodies(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClientWithHistory{
		messageIDs: []string{"msg-a", "msg-b"},
		messageBodies: map[string]string{
			"msg-a": "alpha",
			"msg-b": "bravo",
		},
	}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()

	_, _, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES","TEXT_CHAT_READ_HISTORY"]`)
	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	resp, err := client.GetChatMessagesForBot(botCtx, &botv1.GetChatMessagesForBotRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetMessages(), 2)
	require.NotEmpty(t, resp.GetMessages()[0].GetContent(), "messages[] must include content bodies (app stack6)")
}

func TestAutocompleteSlashOption_pollingBotReturnsPending(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx, botID, _, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)

	manifestYAML := `name: BotC
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: stats
    description: Stats
    options:
      - name: game
        type: string
        required: true
        autocomplete: true
`
	_, err := client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.AutocompleteSlashOption(ctx, &botv1.AutocompleteSlashOptionRequest{
		Chat:         &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:        botID,
		CommandName:  "stats",
		OptionName:   "game",
		FocusedValue: "cs",
	})
	require.NoError(t, err)
	require.True(t, resp.GetPending(), "polling bot autocomplete must return pending=true until CompleteAutocomplete")
}

func TestGetChatMessagesForBot_deniedWhenChatNotWhitelisted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_READ_HISTORY"]`)
	botCtx := withBotToken(context.Background(), botToken)
	otherChat := uuid.New()
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.GetChatMessagesForBot(botCtx, &botv1.GetChatMessagesForBotRequest{
		Chat: &chatv1.ChatRef{Id: otherChat.String(), Type: &chatType},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "not enabled")
}

func TestRehydrateDeferred_registersTokensInHub(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "RehydrateBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botUUID, err := uuid.Parse(reg.GetBot().GetId())
	require.NoError(t, err)

	token := "defer-rehydrate-" + uuid.NewString()
	_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{"ping": true}, token)
	require.NoError(t, err)
	require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
	require.False(t, hub.IsDeferred(token))

	svc := grpcsvc.NewBotGRPC(st, hub)
	svc.RehydrateDeferred(ctx)
	require.True(t, hub.IsDeferred(token), "RehydrateDeferred must register deferred tokens in hub (BOT-C)")
}

func TestSendBotMessage_dmSendDeniedWithoutScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()

	_, _, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)
	dmType := chatv1.ChatType_CHAT_TYPE_DM

	_, err := client.SendBotMessage(botCtx, &botv1.SendBotMessageRequest{
		Chat:    &chatv1.ChatRef{Id: chatID.String(), Type: &dmType},
		Content: "hello",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "DM_SEND")
}

func TestSendBotMessage_dmSendRequiresInteractionContext(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()

	_, _, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`)
	botCtx := withBotToken(context.Background(), botToken)
	dmType := chatv1.ChatType_CHAT_TYPE_DM

	_, err := client.SendBotMessage(botCtx, &botv1.SendBotMessageRequest{
		Chat:    &chatv1.ChatRef{Id: chatID.String(), Type: &dmType},
		Content: "hello",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "interaction context")
}

func TestSendBotMessage_channelPostsWhenWhitelisted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()

	_, _, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	resp, err := client.SendBotMessage(botCtx, &botv1.SendBotMessageRequest{
		Chat:    &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		Content: "broadcast",
	})
	require.NoError(t, err)
	require.Equal(t, "broadcast", resp.GetMessage().GetContent())
	require.Equal(t, "broadcast", msg.lastContent)
}

func TestAssignBotRole_failedPreconditionWhenRoleClientNil(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["MEMBER_ASSIGN_ROLES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.AssignBotRole(botCtx, &botv1.AssignBotRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: uuid.NewString(),
		RoleId:    uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "role client not configured")
}

func TestAssignBotRole_successWithRoleClient(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roleFake := &fakeRoleClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["MEMBER_ASSIGN_ROLES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.AssignBotRole(botCtx, &botv1.AssignBotRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: uuid.NewString(),
		RoleId:    uuid.NewString(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, roleFake.assignCalls)
}

func TestRevokeBotRole_successWithRoleClient(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roleFake := &fakeRoleClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["MEMBER_ASSIGN_ROLES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.RevokeBotRole(botCtx, &botv1.RevokeBotRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: uuid.NewString(),
		RoleId:    uuid.NewString(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, roleFake.revokeCalls)
}

func TestListSpaceMembersForBot_returnsProfileIDs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{space: &fakeSpaceClient{}})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["SPACE_VIEW_MEMBER_LIST"]`)
	botCtx := withBotToken(context.Background(), botToken)

	resp, err := client.ListSpaceMembersForBot(botCtx, &botv1.ListSpaceMembersForBotRequest{
		SpaceId: spaceID.String(),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetProfileIds(), 1)
}

func TestCreateBotChat_failedCreateDoesNotConsumeQuota(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chatFake := &fakeChatClient{createErr: status.Error(codes.Internal, "chat service down")}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{chat: chatFake})
	defer cleanup()

	ctx, botID, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_CREATE_IN_SPACE"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)
	botCtx := withBotToken(context.Background(), botToken)

	_, err = client.CreateBotChat(botCtx, &botv1.CreateBotChatRequest{
		SpaceId:  spaceID.String(),
		Name:     "should-fail",
		ChatType: "channel",
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, 1, chatFake.createChatCalls)

	count, err := st.DailyChatCreateCount(ctx, botUUID)
	require.NoError(t, err)
	require.Equal(t, 0, count, "failed CreateChat must not consume daily quota")
}

func TestCreateBotChat_createsChannelInSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chatFake := &fakeChatClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{chat: chatFake})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_CREATE_IN_SPACE"]`)
	botCtx := withBotToken(context.Background(), botToken)

	resp, err := client.CreateBotChat(botCtx, &botv1.CreateBotChatRequest{
		SpaceId:  spaceID.String(),
		Name:     "audit-log",
		ChatType: "channel",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetChat().GetId())
	require.Equal(t, 1, chatFake.createChatCalls)
}

func TestDeferResponse_marksEventDeferred(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()

	ctx, botID, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)
	botCtx := withBotToken(context.Background(), botToken)

	token := "defer-" + uuid.NewString()
	_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{"x": 1}, token)
	require.NoError(t, err)
	hub.Register(token)

	_, err = client.DeferResponse(botCtx, &botv1.DeferResponseRequest{InteractionToken: token})
	require.NoError(t, err)
	require.True(t, hub.IsDeferred(token))

	var deliveryStatus string
	err = st.Pool.QueryRow(ctx, `
SELECT delivery_status FROM bot_event_log WHERE bot_id = $1 AND interaction_token = $2`,
		botUUID, token).Scan(&deliveryStatus)
	require.NoError(t, err)
	require.Equal(t, "deferred", deliveryStatus)
}

func TestSendBotMessage_deferredFollowUpPostsMessage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()

	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)
	botCtx := withBotToken(context.Background(), botToken)

	token := "defer-msg-" + uuid.NewString()
	_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
		"chat_id":            chatID.String(),
		"invoker_profile_id": uuid.NewString(),
	}, token)
	require.NoError(t, err)
	require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
	hub.RegisterDeferred(token)

	resp, err := client.SendBotMessage(botCtx, &botv1.SendBotMessageRequest{
		InteractionToken: &token,
		Content:          "async pong",
	})
	require.NoError(t, err)
	require.Equal(t, "async pong", resp.GetMessage().GetContent())
	require.False(t, hub.IsDeferred(token))
}

func TestEditBotMessage_updatesViaMessaging(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	resp, err := client.EditBotMessage(botCtx, &botv1.EditBotMessageRequest{
		MessageId: uuid.NewString(),
		Content:   "edited",
	})
	require.NoError(t, err)
	require.Equal(t, "edited", resp.GetMessage().GetContent())
	require.Equal(t, 1, msg.editCalls)
}

func TestSendEphemeral_completesWithoutPersistedMessage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.SendEphemeral(botCtx, &botv1.SendEphemeralRequest{
		Chat:            &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		TargetProfileId: uuid.NewString(),
		Content:         "only you",
	})
	require.NoError(t, err)
}

func TestTouchPresence_rejectsBotIDMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, botID, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.TouchPresence(botCtx, &botv1.TouchPresenceRequest{BotId: uuid.NewString()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = client.TouchPresence(botCtx, &botv1.TouchPresenceRequest{BotId: botID})
	require.NoError(t, err)
}

func TestValidateManifest_acceptsMinimalYAML(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	resp, err := client.ValidateManifest(ctx, &botv1.ValidateManifestRequest{
		ManifestYaml: `name: X
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`,
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetErrors())
}

func TestGetBot_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	_, err := client.GetBot(ctx, &botv1.GetBotRequest{BotId: uuid.NewString()})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestInstallBotInSpace_acceptsPrivilegedScopesWithAck(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roleFake := &fakeRoleClient{}
	client, _, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake})
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name:       "HistoryBot",
		ScopesJson: `["TEXT_CHAT_SEND_MESSAGES","TEXT_CHAT_READ_HISTORY"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	spaceID := uuid.New()
	chatID := uuid.New()
	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	_, err = client.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{
		BotId:   botID,
		SpaceId: spaceID.String(),
		AllowedChats: []*chatv1.ChatRef{
			{Id: chatID.String(), Type: &chatType},
		},
		AcknowledgePrivilegedScopes: true,
	})
	require.NoError(t, err)
}

func TestListBotsInChat_deniedWithoutManageBotsPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roleFake := &fakeRoleClient{denyManageBots: true}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake})
	defer cleanup()

	ctx, botID, _, chatID, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.ListBotsInChat(ctx, &botv1.ListBotsInChatRequest{
		Chat:    &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		SpaceId: spaceID.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_ = botID
}

func TestCompleteInteraction_recoveryWithoutHubWaiter(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()

	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)
	botCtx := withBotToken(context.Background(), botToken)

	token := "recover-" + uuid.NewString()
	_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
		"chat_id":            chatID.String(),
		"invoker_profile_id": uuid.NewString(),
	}, token)
	require.NoError(t, err)
	require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))

	_, err = client.CompleteInteraction(botCtx, &botv1.CompleteInteractionRequest{
		InteractionToken: token,
		Content:          "recovered",
	})
	require.NoError(t, err)
	require.Equal(t, "recovered", msg.lastContent)
}

func TestUninstallBotFromSpace_removesSpaceMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	spaceFake := &fakeSpaceClient{}
	roleFake := &fakeRoleClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake, space: spaceFake})
	defer cleanup()

	ctx, botID, _, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)

	_, err := client.UninstallBotFromSpace(ctx, &botv1.UninstallBotFromSpaceRequest{
		BotId:   botID,
		SpaceId: spaceID.String(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, spaceFake.removeCalls)
}

func TestSetBotChatEnabled_notFoundForUnknownBot(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.SetBotChatEnabled(ctx, &botv1.SetBotChatEnabledRequest{
		BotId:   uuid.NewString(),
		Chat:    &chatv1.ChatRef{Id: uuid.NewString(), Type: &chatType},
		SpaceId: uuid.NewString(),
		Enabled: true,
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestSendBotMessage_completesPendingHubInteraction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	token := "hub-" + uuid.NewString()
	hub.Register(token)

	_, err := client.SendBotMessage(botCtx, &botv1.SendBotMessageRequest{
		InteractionToken: &token,
		Content:          "via hub",
	})
	require.NoError(t, err)
}

func TestAutocompleteSlashOption_pollingBotReturnsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx, botID, _, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)

	manifestYAML := `name: BotC
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: Ping
    options:
      - name: game
        type: string
        required: true
        autocomplete: true
`
	_, err := client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.AutocompleteSlashOption(ctx, &botv1.AutocompleteSlashOptionRequest{
		Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
		OptionName:  "game",
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetChoices())
}

func TestApplyManifest_rejectsInvalidYAML(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "BadManifest", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
		BotId:        reg.GetBot().GetId(),
		ManifestYaml: "not: valid: yaml: [[[",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestExecuteSlashInteraction_offlineBotReturnsUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx, botID, _, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, _ := uuid.Parse(botID)
	_, err := st.Pool.Exec(ctx, `DELETE FROM bot_presence WHERE bot_id = $1`, botUUID)
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
	})
	require.NoError(t, err)
	require.Equal(t, "bot_unavailable", resp.GetErrorCode())
}

func TestExecuteSlashInteraction_unknownCommand(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	_, err := client.TouchPresence(withBotToken(context.Background(), botToken), &botv1.TouchPresenceRequest{BotId: botID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:       botID,
		CommandName: "missing",
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestInstallBotInSpace_groupAddsChatMembers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chatFake := &fakeChatClient{}
	spaceFake := &fakeSpaceClient{}
	roleFake := &fakeRoleClient{}
	client, _, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{chat: chatFake, role: roleFake, space: spaceFake})
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "GroupBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	spaceID := uuid.New()
	chatID := uuid.New()
	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	_, err = client.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{
		BotId:   reg.GetBot().GetId(),
		SpaceId: spaceID.String(),
		AllowedChats: []*chatv1.ChatRef{
			{Id: chatID.String(), Type: &chatType},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, chatFake.addMembersCalls)
}

func TestCompleteInteraction_defersViaHub(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()

	ctx, botID, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)
	botCtx := withBotToken(context.Background(), botToken)

	token := "defer-hub-" + uuid.NewString()
	_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{"x": 1}, token)
	require.NoError(t, err)
	hub.Register(token)

	_, err = client.CompleteInteraction(botCtx, &botv1.CompleteInteractionRequest{
		InteractionToken: token,
		Deferred:         true,
	})
	require.NoError(t, err)
	require.True(t, hub.IsDeferred(token))
}

func TestSendBotMessage_ownerPathViaBotID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()

	ctx, botID, _, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.SendBotMessage(ctx, &botv1.SendBotMessageRequest{
		BotId:   botID,
		Chat:    &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		Content: "owner send",
	})
	require.NoError(t, err)
	require.Equal(t, "owner send", msg.lastContent)
}

func TestSendBotMessage_deniedWhenChatNotWhitelisted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.SendBotMessage(botCtx, &botv1.SendBotMessageRequest{
		Chat:    &chatv1.ChatRef{Id: uuid.NewString(), Type: &chatType},
		Content: "nope",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestExecuteSlashInteraction_deniedWithoutSendScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["SPACE_VIEW_MEMBER_LIST"]`)
	_, err := client.TouchPresence(withBotToken(context.Background(), botToken), &botv1.TouchPresenceRequest{BotId: botID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCreateBotChat_dailyLimitExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chatFake := &fakeChatClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{chat: chatFake})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_CREATE_IN_SPACE"]`)
	botCtx := withBotToken(context.Background(), botToken)

	for i := 0; i < 10; i++ {
		_, err := client.CreateBotChat(botCtx, &botv1.CreateBotChatRequest{
			SpaceId:  spaceID.String(),
			Name:     "ch",
			ChatType: "channel",
		})
		require.NoError(t, err)
	}
	_, err := client.CreateBotChat(botCtx, &botv1.CreateBotChatRequest{
		SpaceId:  spaceID.String(),
		Name:     "one-too-many",
		ChatType: "channel",
	})
	require.Error(t, err)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestCreateBotChat_concurrentDailyLimit(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chatFake := &fakeChatClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{chat: chatFake})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_CREATE_IN_SPACE"]`)
	botCtx := withBotToken(context.Background(), botToken)

	const workers = 15
	var wg sync.WaitGroup
	var mu sync.Mutex
	var successes int
	var exhausted int
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := client.CreateBotChat(botCtx, &botv1.CreateBotChatRequest{
				SpaceId:  spaceID.String(),
				Name:     fmt.Sprintf("concurrent-%d", idx),
				ChatType: "channel",
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successes++
				return
			}
			if status.Code(err) == codes.ResourceExhausted {
				exhausted++
				return
			}
			t.Errorf("unexpected error: %v", err)
		}(i)
	}
	wg.Wait()
	require.Equal(t, 10, successes, "multi-replica race must allow at most 10 successful creates")
	require.Equal(t, workers-10, exhausted)
}

func TestRegisterBot_rejectsInvalidScopesJSON(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	_, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "BadScopes", ScopesJson: `not-json`,
	})
	require.Error(t, err)
}

func TestListSpaceMembersForBot_failedPreconditionWithoutSpaceClient(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["SPACE_VIEW_MEMBER_LIST"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.ListSpaceMembersForBot(botCtx, &botv1.ListSpaceMembersForBotRequest{
		SpaceId: spaceID.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestExecuteSlashInteraction_webhookDeferredResponse(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"deferred":true}`))
	}))
	defer srv.Close()

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "DeferWebhookBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botToken := reg.GetTokenResponse().GetToken()

	manifestYAML := `name: DeferWebhookBot
webhook_url: ` + srv.URL + `
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	_, err = st.InstallInSpace(ctx, botUUID, uuid.New(), profile, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	_, err = client.TouchPresence(withBotToken(context.Background(), botToken), &botv1.TouchPresenceRequest{BotId: botID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
	})
	require.NoError(t, err)
	require.True(t, resp.GetDeferred())
}

func TestExecuteSlashInteraction_webhookSuccessWithContent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":"webhook-pong"}`))
	}))
	defer srv.Close()

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "WebhookBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botToken := reg.GetTokenResponse().GetToken()

	manifestYAML := `name: WebhookBot
webhook_url: ` + srv.URL + `
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	_, err = st.InstallInSpace(ctx, botUUID, uuid.New(), profile, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	_, err = client.TouchPresence(withBotToken(context.Background(), botToken), &botv1.TouchPresenceRequest{BotId: botID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
	})
	require.NoError(t, err)
	require.Equal(t, "webhook-pong", resp.GetMessage().GetContent())
	require.Equal(t, "webhook-pong", msg.lastContent)
}

func TestRegisterBot_unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()

	_, err := client.RegisterBot(context.Background(), &botv1.RegisterBotRequest{
		Name: "NoAuth", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestRevokeBotRole_failedPreconditionWhenRoleClientNil(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["MEMBER_ASSIGN_ROLES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.RevokeBotRole(botCtx, &botv1.RevokeBotRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: uuid.NewString(),
		RoleId:    uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCompleteInteraction_unknownToken(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.CompleteInteraction(botCtx, &botv1.CompleteInteractionRequest{
		InteractionToken: "missing-token",
		Content:          "nope",
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestExecuteSlashInteraction_deniedWhenChatNotWhitelisted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx, botID, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	_, err := client.TouchPresence(withBotToken(context.Background(), botToken), &botv1.TouchPresenceRequest{BotId: botID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: uuid.NewString(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCreateBotChat_rejectsEmptyName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chatFake := &fakeChatClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{chat: chatFake})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_CREATE_IN_SPACE"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.CreateBotChat(botCtx, &botv1.CreateBotChatRequest{
		SpaceId:  spaceID.String(),
		Name:     "   ",
		ChatType: "group",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestValidateManifest_reportsErrors(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	resp, err := client.ValidateManifest(ctx, &botv1.ValidateManifestRequest{
		ManifestYaml: `name: ""
scopes: []
`,
	})
	require.NoError(t, err)
	require.False(t, resp.GetValid())
	require.NotEmpty(t, resp.GetErrors())
}

func TestListSpaceMembersForBot_honorsCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{space: &fakeSpaceClient{}})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["SPACE_VIEW_MEMBER_LIST"]`)
	botCtx := withBotToken(context.Background(), botToken)
	cursor := "page-2"

	resp, err := client.ListSpaceMembersForBot(botCtx, &botv1.ListSpaceMembersForBotRequest{
		SpaceId: spaceID.String(),
		Cursor:  &cursor,
	})
	require.NoError(t, err)
	require.Len(t, resp.GetProfileIds(), 1)
}

func TestInstallBotInSpace_unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: &fakeRoleClient{}})
	defer cleanup()

	_, err := client.InstallBotInSpace(context.Background(), &botv1.InstallBotInSpaceRequest{
		BotId:   uuid.NewString(),
		SpaceId: uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCreateBotChat_failedPreconditionWithoutChatClient(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_CREATE_IN_SPACE"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.CreateBotChat(botCtx, &botv1.CreateBotChatRequest{
		SpaceId:  spaceID.String(),
		Name:     "logs",
		ChatType: "channel",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestSendBotMessage_deferredRequiresContent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()

	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)
	botCtx := withBotToken(context.Background(), botToken)

	token := "defer-empty-" + uuid.NewString()
	_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{
		"chat_id": chatID.String(),
	}, token)
	require.NoError(t, err)
	require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))
	hub.RegisterDeferred(token)

	_, err = client.SendBotMessage(botCtx, &botv1.SendBotMessageRequest{
		InteractionToken: &token,
		Content:          "   ",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestUpdateBot_deniedForNonOwner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()

	ownerCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ownerCtx, &botv1.RegisterBotRequest{
		Name: "OwnedBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	otherCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	name := "Hijack"
	_, err = client.UpdateBot(otherCtx, &botv1.UpdateBotRequest{
		BotId: reg.GetBot().GetId(),
		Name:  &name,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestSendEphemeral_deniedWhenChatNotWhitelisted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL

	_, err := client.SendEphemeral(botCtx, &botv1.SendEphemeralRequest{
		Chat:            &chatv1.ChatRef{Id: uuid.NewString(), Type: &chatType},
		TargetProfileId: uuid.NewString(),
		Content:         "secret",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestEditBotMessage_requiresContent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.EditBotMessage(botCtx, &botv1.EditBotMessageRequest{
		MessageId: uuid.NewString(),
		Content:   "  ",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRegisterCommands_rejectsInvalidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "CmdBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	_, err = client.RegisterCommands(ctx, &botv1.RegisterCommandsRequest{
		BotId:        reg.GetBot().GetId(),
		CommandsJson: `{`,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestInstallBotInSpace_deniedWithoutManageBotsPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roleFake := &fakeRoleClient{denyManageBots: true}
	client, _, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake})
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "InstallBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	_, err = client.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{
		BotId:   reg.GetBot().GetId(),
		SpaceId: uuid.NewString(),
		AllowedChats: []*chatv1.ChatRef{
			{Id: uuid.NewString(), Type: &chatType},
		},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestUninstallBotFromSpace_deniedWithoutManageBotsPermission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roleFake := &fakeRoleClient{denyManageBots: true}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake})
	defer cleanup()

	ctx, botID, _, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)

	_, err := client.UninstallBotFromSpace(ctx, &botv1.UninstallBotFromSpaceRequest{
		BotId:   botID,
		SpaceId: spaceID.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestExecuteSlashInteraction_webhookEphemeralResponse(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":"private","ephemeral":true}`))
	}))
	defer srv.Close()

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "EphWebhookBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botToken := reg.GetTokenResponse().GetToken()

	manifestYAML := `name: EphWebhookBot
webhook_url: ` + srv.URL + `
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	_, err = st.InstallInSpace(ctx, botUUID, uuid.New(), profile, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	_, err = client.TouchPresence(withBotToken(context.Background(), botToken), &botv1.TouchPresenceRequest{BotId: botID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
	})
	require.NoError(t, err)
	require.True(t, resp.GetIsEphemeral())
	require.Equal(t, "private", resp.GetContent())
}

func TestRegisterBot_actorProfileProvisionFailure(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	user := &fakeUserClient{createErr: fmt.Errorf("user down")}
	client, _, cleanup := startBotGRPCWithDeps(t, user, nil)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	_, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "FailActor", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestDeleteBot_deniedForNonOwner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()

	ownerCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ownerCtx, &botv1.RegisterBotRequest{
		Name: "DeleteMe", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	otherCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	_, err = client.DeleteBot(otherCtx, &botv1.DeleteBotRequest{BotId: reg.GetBot().GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetBot_deniedForNonOwner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()

	ownerCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ownerCtx, &botv1.RegisterBotRequest{
		Name: "SecretBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	otherCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	_, err = client.GetBot(otherCtx, &botv1.GetBotRequest{BotId: reg.GetBot().GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestRegenerateToken_deniedForNonOwner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()

	ownerCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ownerCtx, &botv1.RegisterBotRequest{
		Name: "TokenBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	otherCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	_, err = client.RegenerateToken(otherCtx, &botv1.RegenerateTokenRequest{BotId: reg.GetBot().GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestRegisterCommands_rejectsInvalidCommandDefinition(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "CmdBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	_, err = client.RegisterCommands(ctx, &botv1.RegisterCommandsRequest{
		BotId:        reg.GetBot().GetId(),
		CommandsJson: `[{"name":"","description":"bad"}]`,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRegenerateToken_botNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	_, err := client.RegenerateToken(ctx, &botv1.RegenerateTokenRequest{BotId: uuid.NewString()})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestEditBotMessage_requiresMessageId(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg})
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.EditBotMessage(botCtx, &botv1.EditBotMessageRequest{
		MessageId: "  ",
		Content:   "edited",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestEditBotMessage_failedPreconditionWithoutMessaging(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.EditBotMessage(botCtx, &botv1.EditBotMessageRequest{
		MessageId: uuid.NewString(),
		Content:   "edited",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestGetBotBySlug_returnsRegisteredBot(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "SlugBot", Description: "by slug", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	slug := reg.GetBot().GetSlug()
	require.NotEmpty(t, slug)

	got, err := client.GetBotBySlug(ctx, &botv1.GetBotBySlugRequest{Slug: slug})
	require.NoError(t, err)
	require.Equal(t, reg.GetBot().GetId(), got.GetBot().GetId())
	require.Equal(t, "SlugBot", got.GetBot().GetName())
	require.Equal(t, "by slug", got.GetBot().GetDescription())
}

func TestGetBotBySlug_invalidArgumentWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	_, err := client.GetBotBySlug(ctx, &botv1.GetBotBySlugRequest{Slug: "  "})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetBotBySlug_notFoundForUnknownSlug(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	_, err := client.GetBotBySlug(ctx, &botv1.GetBotBySlugRequest{Slug: "missing-bot-slug"})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

type fakeMessagingClientWithHistory struct {
	fakeMessagingClient
	getMessagesCalls int
	messageIDs       []string
	messageBodies    map[string]string
}

func (f *fakeMessagingClientWithHistory) GetMessages(_ context.Context, _ *messagingv1.GetMessagesRequest) (*messagingv1.GetMessagesResponse, error) {
	f.getMessagesCalls++
	msgs := make([]*messagingv1.Message, 0, len(f.messageIDs))
	for _, id := range f.messageIDs {
		content := id
		if f.messageBodies != nil {
			if body, ok := f.messageBodies[id]; ok {
				content = body
			}
		}
		msgs = append(msgs, &messagingv1.Message{Id: id, Content: content})
	}
	return &messagingv1.GetMessagesResponse{
		MessageList: &messagingv1.MessageList{Messages: msgs},
	}, nil
}

func callBotGRPCMethod(t *testing.T, svc *grpcsvc.BotGRPC, ctx context.Context, method string, req any) (any, error) {
	t.Helper()
	m := reflect.ValueOf(svc).MethodByName(method)
	require.True(t, m.IsValid(), "%s must be implemented on BotGRPC (BOT-C)", method)
	args := []reflect.Value{reflect.ValueOf(ctx)}
	if m.Type().NumIn() == 2 {
		reqVal := reflect.ValueOf(req)
		if req == nil {
			reqVal = reflect.New(m.Type().In(1).Elem())
		} else if reqVal.Kind() == reflect.Ptr && reqVal.Type() != m.Type().In(1) {
			reqVal = reflect.ValueOf(req).Convert(m.Type().In(1))
		}
		args = append(args, reqVal)
	}
	out := m.Call(args)
	if len(out) > 1 && !out[1].IsNil() {
		return nil, out[1].Interface().(error)
	}
	if len(out) > 0 && out[0].CanInterface() {
		return out[0].Interface(), nil
	}
	return nil, nil
}

func wireBotCServiceRole(t *testing.T, svc *grpcsvc.BotGRPC, role rolev1.RoleServiceServer) {
	t.Helper()
	rl := bufconn.Listen(1024)
	rs := grpc.NewServer()
	rolev1.RegisterRoleServiceServer(rs, role)
	go func() { _ = rs.Serve(rl) }()
	rconn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return rl.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	svc.Role = rolev1.NewRoleServiceClient(rconn)
}

func TestCreateBotRole_requiresSpaceManageRolesScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, ok := reflect.TypeOf(&grpcsvc.BotGRPC{}).MethodByName("CreateBotRole")
	require.True(t, ok, "CreateBotRole must exist on BotGRPC (BOT-C)")

	roleFake := &fakeRoleClient{}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	svc := grpcsvc.NewBotGRPC(st, hub)
	wireBotCServiceRole(t, svc, roleFake)

	m, _ := reflect.TypeOf(&grpcsvc.BotGRPC{}).MethodByName("CreateBotRole")
	req := reflect.New(m.Type.In(2).Elem())
	if f := req.Elem().FieldByName("SpaceId"); f.IsValid() {
		f.SetString(spaceID.String())
	}
	if f := req.Elem().FieldByName("Name"); f.IsValid() {
		f.SetString("Helper")
	}

	_, err := callBotGRPCMethod(t, svc, botCtx, "CreateBotRole", req.Interface())
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err),
		"CreateBotRole must require SPACE_MANAGE_ROLES privileged scope (BOT-C)")
}

func TestAutocompleteSlashOption_pollingBotReturnsChoicesAfterComplete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()
	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	manifestYAML := `name: BotC
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: stats
    description: Stats
    options:
      - name: game
        type: string
        required: true
        autocomplete: true
`
	_, err := client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	acReq := &botv1.AutocompleteSlashOptionRequest{
		Chat:         &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:        botID,
		CommandName:  "stats",
		OptionName:   "game",
		FocusedValue: "cs",
	}

	resp, err := client.AutocompleteSlashOption(ctx, acReq)
	require.NoError(t, err)
	require.Empty(t, resp.GetChoices(), "polling bot starts with empty choices until bot completes")

	svc := grpcsvc.NewBotGRPC(st, hub)
	_, ok := reflect.TypeOf(&grpcsvc.BotGRPC{}).MethodByName("CompleteAutocomplete")
	require.True(t, ok, "CompleteAutocomplete must exist on BotGRPC (BOT-C)")

	completeMethod, _ := reflect.TypeOf(&grpcsvc.BotGRPC{}).MethodByName("CompleteAutocomplete")
	completeReq := reflect.New(completeMethod.Type.In(2).Elem())
	if f := completeReq.Elem().FieldByName("RequestId"); f.IsValid() {
		f.SetString("ac-req-1")
	}
	choicesField := completeReq.Elem().FieldByName("Choices")
	if choicesField.IsValid() && choicesField.Type().Kind() == reflect.Slice {
		choiceType := choicesField.Type().Elem()
		if choiceType.Kind() == reflect.Ptr {
			choiceType = choiceType.Elem()
		}
		choice := reflect.New(choiceType).Elem()
		if n := choice.FieldByName("Name"); n.IsValid() {
			n.SetString("CS2")
		}
		if v := choice.FieldByName("Value"); v.IsValid() {
			v.SetString("cs2")
		}
		if choicesField.Type().Elem().Kind() == reflect.Ptr {
			choicesField.Set(reflect.Append(choicesField, choice.Addr()))
		} else {
			choicesField.Set(reflect.Append(choicesField, choice))
		}
	}
	_, err = callBotGRPCMethod(t, svc, botCtx, "CompleteAutocomplete", completeReq.Interface())
	require.NoError(t, err)

	resp, err = client.AutocompleteSlashOption(ctx, acReq)
	require.NoError(t, err)
	require.Len(t, resp.GetChoices(), 1, "polling autocomplete must return choices after CompleteAutocomplete (BOT-C)")
	require.Equal(t, "CS2", resp.GetChoices()[0].GetName())
}

func TestDeferredTTLSweeper_abandonsStaleDeferredRows(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()

	ctx, botID, _, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)

	token := "defer-stale-" + uuid.NewString()
	_, err = st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{"x": 1}, token)
	require.NoError(t, err)
	require.NoError(t, st.MarkEventDeferred(ctx, botUUID, token))

	_, err = st.Pool.Exec(ctx, `
UPDATE bot_event_log SET created_at = now() - interval '25 hours'
WHERE bot_id = $1 AND interaction_token = $2`, botUUID, token)
	require.NoError(t, err)

	svc := grpcsvc.NewBotGRPC(st, hub)
	_, ok := reflect.TypeOf(&grpcsvc.BotGRPC{}).MethodByName("RunDeferredTTLSweeper")
	require.True(t, ok, "RunDeferredTTLSweeper must exist on BotGRPC (BOT-C)")
	_, err = callBotGRPCMethod(t, svc, ctx, "RunDeferredTTLSweeper", nil)
	require.NoError(t, err)

	var deliveryStatus string
	err = st.Pool.QueryRow(ctx, `
SELECT delivery_status FROM bot_event_log WHERE bot_id = $1 AND interaction_token = $2`,
		botUUID, token).Scan(&deliveryStatus)
	require.NoError(t, err)
	require.Equal(t, "abandoned", deliveryStatus,
		"deferred TTL sweeper must abandon stale deferred rows (BOT-C)")
}

func TestCreateBotRole_createsRoleWithManageRolesScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roleFake := &fakeRoleClient{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: roleFake})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["SPACE_MANAGE_ROLES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	resp, err := client.CreateBotRole(botCtx, &botv1.CreateBotRoleRequest{
		SpaceId:         spaceID.String(),
		Name:            "Helper",
		PermissionsMask: 1,
		Position:        2,
	})
	require.NoError(t, err)
	require.Equal(t, "Helper", resp.GetRole().GetName())
}

func TestCreateBotRole_requiresName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: &fakeRoleClient{}})
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["SPACE_MANAGE_ROLES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.CreateBotRole(botCtx, &botv1.CreateBotRoleRequest{
		SpaceId: spaceID.String(),
		Name:    "  ",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCompleteAutocomplete_unknownRequestID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.CompleteAutocomplete(botCtx, &botv1.CompleteAutocompleteRequest{
		RequestId: "missing-ac-req",
		Choices:   []*botv1.AutocompleteChoice{{Name: "X", Value: "x"}},
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestCompleteAutocomplete_requiresRequestID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, nil)
	defer cleanup()

	_, _, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.CompleteAutocomplete(botCtx, &botv1.CompleteAutocompleteRequest{
		RequestId: "  ",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListSlashCommands_usesPresenceTTLFromEnv(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Setenv("BOT_PRESENCE_TTL", "120")
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx, botID, _, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	resp, err := client.ListSlashCommandsForChat(ctx, &botv1.ListSlashCommandsForChatRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetCommands())
	require.True(t, resp.GetCommands()[0].GetOnline())
}
