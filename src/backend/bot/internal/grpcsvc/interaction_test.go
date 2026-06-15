package grpcsvc_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	userv1 "voice.app/voice/user/v1"
)

type fakeUserClient struct {
	userv1.UnimplementedUserServiceServer
	profileID string
}

func (f *fakeUserClient) CreateProfile(context.Context, *userv1.CreateProfileRequest) (*userv1.CreateProfileResponse, error) {
	return &userv1.CreateProfileResponse{
		Profile: &userv1.Profile{Id: f.profileID, DisplayName: "Bot"},
	}, nil
}

type fakeMessagingClient struct {
	messagingv1.UnimplementedMessagingServiceServer
	lastContent string
}

func (f *fakeMessagingClient) SendMessage(_ context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	f.lastContent = req.GetContent()
	return &messagingv1.SendMessageResponse{
		Message: &messagingv1.Message{
			Id:      uuid.NewString(),
			Content: req.GetContent(),
		},
	}, nil
}

func startBotGRPCWithDeps(t *testing.T, user *fakeUserClient, msg *fakeMessagingClient) (botv1.BotServiceClient, *store.BotStore, func()) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "botinteraction", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)

	st := &store.BotStore{Pool: pool}
	hub := dispatch.NewHub()
	svc := grpcsvc.NewBotGRPC(st, hub)
	if user != nil {
		ul := bufconn.Listen(1024)
		us := grpc.NewServer()
		userv1.RegisterUserServiceServer(us, user)
		go func() { _ = us.Serve(ul) }()
		uconn, err := grpc.NewClient("passthrough:///bufnet",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return ul.Dial() }),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		require.NoError(t, err)
		svc.User = userv1.NewUserServiceClient(uconn)
	}
	if msg != nil {
		ml := bufconn.Listen(1024)
		ms := grpc.NewServer()
		messagingv1.RegisterMessagingServiceServer(ms, msg)
		go func() { _ = ms.Serve(ml) }()
		mconn, err := grpc.NewClient("passthrough:///bufnet",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return ml.Dial() }),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		require.NoError(t, err)
		svc.Messaging = messagingv1.NewMessagingServiceClient(mconn)
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
	return botv1.NewBotServiceClient(conn), st, func() { s.Stop(); _ = conn.Close() }
}

func withBotToken(ctx context.Context, token string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs(authctx.HeaderBotToken, token))
}

func TestRegisterBot_provisionsActorProfile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	actorID := uuid.NewString()
	user := &fakeUserClient{profileID: actorID}
	client, st, cleanup := startBotGRPCWithDeps(t, user, nil)
	defer cleanup()

	owner := uuid.New()
	profile := uuid.New()
	ctx := withAccount(context.Background(), owner, profile)
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "StatsBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	require.NotEmpty(t, reg.GetTokenResponse().GetToken())

	botUUID, err := uuid.Parse(reg.GetBot().GetId())
	require.NoError(t, err)
	row, err := st.GetBotByID(context.Background(), botUUID)
	require.NoError(t, err)
	require.Equal(t, actorID, row.ActorProfileID.String())
}

func TestExecuteSlashInteraction_pollingPersistsMessage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "PingBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botToken := reg.GetTokenResponse().GetToken()

	manifestYAML := `name: PingBot
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	profile, _ := authProfile(ctx)
	_, err = st.InstallInSpace(ctx, botUUID, uuid.New(), profile, []uuid.UUID{chatID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	respCh := make(chan *botv1.ExecuteSlashInteractionResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
			Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
			BotId:       botID,
			CommandName: "ping",
		})
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	deadline := time.Now().Add(3 * time.Second)
	var token string
	for time.Now().Before(deadline) {
		_, _, payloads, err := st.ListPendingEvents(context.Background(), botUUID, 5)
		if err == nil && len(payloads) > 0 {
			var m map[string]any
			_ = json.Unmarshal([]byte(payloads[0]), &m)
			if v, ok := m["interaction_token"].(string); ok && v != "" {
				token = v
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NotEmpty(t, token)

	botCtx := withBotToken(context.Background(), botToken)
	_, err = client.CompleteInteraction(botCtx, &botv1.CompleteInteractionRequest{
		InteractionToken: token,
		Content:          "pong",
		IsEphemeral:      false,
	})
	require.NoError(t, err)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case resp := <-respCh:
		require.NotNil(t, resp.GetMessage())
		require.Equal(t, "pong", resp.GetMessage().GetContent())
		require.Equal(t, "pong", msg.lastContent)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for slash interaction")
	}
}
