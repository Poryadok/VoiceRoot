package grpcsvc_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

type fakeRoleClient struct {
	rolev1.UnimplementedRoleServiceServer
	assignCalls    int
	revokeCalls    int
	denyManageBots bool
}

func (f *fakeRoleClient) CheckPermission(_ context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	if f.denyManageBots && req.GetPermissionName() == "SPACE_MANAGE_BOTS" {
		return &rolev1.CheckPermissionResponse{Allowed: false}, nil
	}
	return &rolev1.CheckPermissionResponse{Allowed: true}, nil
}

func (f *fakeRoleClient) AssignRole(_ context.Context, _ *rolev1.AssignRoleRequest) (*rolev1.AssignRoleResponse, error) {
	f.assignCalls++
	return &rolev1.AssignRoleResponse{}, nil
}

func (f *fakeRoleClient) RevokeRole(_ context.Context, _ *rolev1.RevokeRoleRequest) (*rolev1.RevokeRoleResponse, error) {
	f.revokeCalls++
	return &rolev1.RevokeRoleResponse{}, nil
}

type fakeSpaceClient struct {
	spacev1.UnimplementedSpaceServiceServer
	removeCalls int
}

func (f *fakeSpaceClient) ListMembers(_ context.Context, _ *spacev1.ListMembersRequest) (*spacev1.ListMembersResponse, error) {
	return &spacev1.ListMembersResponse{
		SpaceMemberList: &spacev1.SpaceMemberList{
			Members: []*spacev1.SpaceMembership{{ProfileId: uuid.NewString()}},
		},
	}, nil
}

func (f *fakeSpaceClient) RemoveBotMember(_ context.Context, _ *spacev1.RemoveBotMemberRequest) (*spacev1.RemoveBotMemberResponse, error) {
	f.removeCalls++
	return &spacev1.RemoveBotMemberResponse{}, nil
}

func (f *fakeSpaceClient) AddBotMember(_ context.Context, _ *spacev1.AddBotMemberRequest) (*spacev1.AddBotMemberResponse, error) {
	return &spacev1.AddBotMemberResponse{}, nil
}

type fakeChatClient struct {
	chatv1.UnimplementedChatServiceServer
	addMembersCalls int
	createChatCalls int
}

func (f *fakeChatClient) AddMembers(_ context.Context, req *chatv1.AddMembersRequest) (*chatv1.AddMembersResponse, error) {
	f.addMembersCalls++
	return &chatv1.AddMembersResponse{}, nil
}

func (f *fakeChatClient) CreateChat(_ context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	f.createChatCalls++
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	if req.GetType() == chatv1.ChatType_CHAT_TYPE_GROUP {
		chatType = chatv1.ChatType_CHAT_TYPE_GROUP
	}
	return &chatv1.CreateChatResponse{
		Chat: &chatv1.Chat{Id: uuid.NewString(), Type: chatType},
	}, nil
}

type botCDeps struct {
	user  *fakeUserClient
	msg   messagingv1.MessagingServiceServer
	chat  *fakeChatClient
	role  rolev1.RoleServiceServer
	space *fakeSpaceClient
}

func startBotGRPCWithBotCDeps(t *testing.T, deps *botCDeps) (botv1.BotServiceClient, *store.BotStore, *dispatch.Hub, func()) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "botc", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)

	st := &store.BotStore{Pool: pool}
	hub := dispatch.NewHub()
	svc := grpcsvc.NewBotGRPC(st, hub)
	if deps != nil {
		if deps.user != nil {
			ul := bufconn.Listen(1024)
			us := grpc.NewServer()
			userv1.RegisterUserServiceServer(us, deps.user)
			go func() { _ = us.Serve(ul) }()
			uconn, err := grpc.NewClient("passthrough:///bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return ul.Dial() }),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			svc.User = userv1.NewUserServiceClient(uconn)
		}
		if deps.msg != nil {
			ml := bufconn.Listen(1024)
			ms := grpc.NewServer()
			messagingv1.RegisterMessagingServiceServer(ms, deps.msg)
			go func() { _ = ms.Serve(ml) }()
			mconn, err := grpc.NewClient("passthrough:///bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return ml.Dial() }),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			svc.Messaging = messagingv1.NewMessagingServiceClient(mconn)
		}
		if deps.chat != nil {
			cl := bufconn.Listen(1024)
			cs := grpc.NewServer()
			chatv1.RegisterChatServiceServer(cs, deps.chat)
			go func() { _ = cs.Serve(cl) }()
			cconn, err := grpc.NewClient("passthrough:///bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return cl.Dial() }),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			svc.Chat = chatv1.NewChatServiceClient(cconn)
		}
		if deps.role != nil {
			rl := bufconn.Listen(1024)
			rs := grpc.NewServer()
			rolev1.RegisterRoleServiceServer(rs, deps.role)
			go func() { _ = rs.Serve(rl) }()
			rconn, err := grpc.NewClient("passthrough:///bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return rl.Dial() }),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			svc.Role = rolev1.NewRoleServiceClient(rconn)
		}
		if deps.space != nil {
			sl := bufconn.Listen(1024)
			ss := grpc.NewServer()
			spacev1.RegisterSpaceServiceServer(ss, deps.space)
			go func() { _ = ss.Serve(sl) }()
			sconn, err := grpc.NewClient("passthrough:///bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return sl.Dial() }),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			svc.Space = spacev1.NewSpaceServiceClient(sconn)
		}
	}

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	botv1.RegisterBotServiceServer(s, svc)
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return botv1.NewBotServiceClient(conn), st, hub, func() { s.Stop(); _ = conn.Close() }
}

func setupBotCCommandBot(
	t *testing.T,
	client botv1.BotServiceClient,
	st *store.BotStore,
	scopesJSON string,
) (context.Context, string, string, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name:       "BotC",
		ScopesJson: scopesJSON,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botToken := reg.GetTokenResponse().GetToken()

	manifestYAML := manifestYAMLForBotC(scopesJSON)
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	spaceID := uuid.New()
	chatID := uuid.New()
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, botUUID, spaceID, profile, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botUUID))
	return ctx, botID, botToken, chatID, spaceID
}

func manifestYAMLForBotC(scopesJSON string) string {
	var scopes []string
	if err := json.Unmarshal([]byte(scopesJSON), &scopes); err != nil || len(scopes) == 0 {
		scopes = []string{"TEXT_CHAT_SEND_MESSAGES"}
	}
	var b strings.Builder
	b.WriteString("name: BotC\nscopes:\n")
	for _, scope := range scopes {
		b.WriteString("  - ")
		b.WriteString(scope)
		b.WriteByte('\n')
	}
	b.WriteString(`commands:
  - name: ping
    description: Ping
`)
	return b.String()
}

func botCProtoMessage(t *testing.T, name string) proto.Message {
	t.Helper()
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName("voice.bot.v1." + name))
	if err != nil {
		t.Skipf("voice.bot.v1.%s not in generated proto yet (BOT-C)", name)
	}
	return mt.New().Interface().(proto.Message)
}

func setProtoStringField(t *testing.T, msg proto.Message, field, value string) {
	t.Helper()
	pr := msg.ProtoReflect()
	fd := pr.Descriptor().Fields().ByName(protoreflect.Name(field))
	if fd == nil {
		t.Fatalf("field %q missing on %s (BOT-C proto drift)", field, pr.Descriptor().FullName())
	}
	pr.Set(fd, protoreflect.ValueOfString(value))
}

func invokeBotClientMethod(t *testing.T, client botv1.BotServiceClient, method string, req proto.Message) error {
	t.Helper()
	clientVal := reflect.ValueOf(client)
	m := clientVal.MethodByName(method)
	if !m.IsValid() {
		t.Skipf("BotServiceClient.%s not generated yet (BOT-C)", method)
	}
	ctx := context.Background()
	out := m.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(req)})
	if len(out) != 2 {
		t.Fatalf("unexpected %s signature", method)
	}
	if errVal := out[1]; !errVal.IsNil() {
		return errVal.Interface().(error)
	}
	return nil
}

func TestListSlashCommandsForChat_offlineWithoutPresence(t *testing.T) {
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
	list, err := client.ListSlashCommandsForChat(ctx, &botv1.ListSlashCommandsForChatRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
	})
	require.NoError(t, err)
	require.Len(t, list.GetCommands(), 1)
	require.False(t, list.GetCommands()[0].GetOnline(), "bot without presence heartbeat must be offline (BOT-C)")
}

func TestListSlashCommandsForChat_onlineAfterTouchPresence(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx, botID, botToken, chatID, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.TouchPresence(botCtx, &botv1.TouchPresenceRequest{BotId: botID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	list, err := client.ListSlashCommandsForChat(ctx, &botv1.ListSlashCommandsForChatRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
	})
	require.NoError(t, err)
	require.Len(t, list.GetCommands(), 1)
	require.True(t, list.GetCommands()[0].GetOnline(), "bot must be online after TouchPresence (BOT-C)")
}

func TestInstallBotInSpace_rejectsPrivilegedScopeWithoutAcknowledge(t *testing.T) {
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
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	req := &botv1.InstallBotInSpaceRequest{
		BotId:   botID,
		SpaceId: spaceID.String(),
		AllowedChats: []*chatv1.ChatRef{
			{Id: chatID.String(), Type: &chatType},
		},
	}
	if fd := req.ProtoReflect().Descriptor().Fields().ByName("acknowledge_privileged_scopes"); fd != nil {
		req.ProtoReflect().Set(fd, protoreflect.ValueOfBool(false))
	}

	_, err = client.InstallBotInSpace(ctx, req)
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err),
		"TEXT_CHAT_READ_HISTORY install must require acknowledge_privileged_scopes (BOT-C)")
}

func TestAssignBotRole_deniedWithoutMemberAssignRolesScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.AssignBotRole(botCtx, &botv1.AssignBotRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: uuid.NewString(),
		RoleId:    uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err),
		"AssignBotRole must require MEMBER_ASSIGN_ROLES scope (BOT-C)")
}

func TestRevokeBotRole_deniedWithoutMemberAssignRolesScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.RevokeBotRole(botCtx, &botv1.RevokeBotRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: uuid.NewString(),
		RoleId:    uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err),
		"RevokeBotRole must require MEMBER_ASSIGN_ROLES scope (BOT-C)")
}

func TestListSpaceMembersForBot_deniedWithoutSpaceViewMemberListScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.ListSpaceMembersForBot(botCtx, &botv1.ListSpaceMembersForBotRequest{
		SpaceId: spaceID.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err),
		"ListSpaceMembersForBot must require SPACE_VIEW_MEMBER_LIST scope (BOT-C)")
}

func TestCreateBotChat_deniedWithoutTextChatCreateInSpaceScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	_, _, botToken, _, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botCtx := withBotToken(context.Background(), botToken)

	_, err := client.CreateBotChat(botCtx, &botv1.CreateBotChatRequest{
		SpaceId:  spaceID.String(),
		Name:     "mod-log",
		ChatType: "channel",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err),
		"CreateBotChat must require TEXT_CHAT_CREATE_IN_SPACE scope (BOT-C)")
}

func TestInstallBotInSpace_channelSkipsChatAddMembersWhenSpaceAddBotMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chatFake := &fakeChatClient{}
	roleFake := &fakeRoleClient{}
	client, _, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{chat: chatFake, role: roleFake})
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name:       "ChannelBot",
		ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	spaceID := uuid.New()
	chatID := uuid.New()
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{
		BotId:   botID,
		SpaceId: spaceID.String(),
		AllowedChats: []*chatv1.ChatRef{
			{Id: chatID.String(), Type: &chatType},
		},
	})
	require.NoError(t, err)

	require.Equal(t, 0, chatFake.addMembersCalls,
		"channel install must skip Chat.AddMembers when Space.AddBotMember handles membership (BOT-C)")
}

func TestPollEvents_marksEventDeliveredAfterStream(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx, botID, botToken, _, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)

	eventID, err := st.EnqueueEvent(ctx, botUUID, "interaction", map[string]any{"ping": true}, "")
	require.NoError(t, err)

	botCtx := withBotToken(context.Background(), botToken)
	stream, err := client.PollEvents(botCtx, &botv1.PollEventsRequest{BotId: botID})
	require.NoError(t, err)

	var received bool
	for {
		resp, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		if resp.GetBotEvent().GetEventId() == eventID.String() {
			received = true
			break
		}
	}
	require.True(t, received, "PollEvents must stream pending bot events")

	var deliveryStatus string
	err = st.Pool.QueryRow(ctx, `
SELECT delivery_status FROM bot_event_log WHERE id = $1`, eventID).Scan(&deliveryStatus)
	require.NoError(t, err)
	require.Equal(t, "delivered", deliveryStatus,
		"PollEvents must mark streamed events delivered (BOT-C)")
}
