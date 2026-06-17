package grpcsvc_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func migrationSQL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db")
	var b strings.Builder
	for _, name := range []string{"000001_init.up.sql", "000002_bot_presence.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String()
}

func startBotGRPC(t *testing.T) (botv1.BotServiceClient, *store.BotStore, func()) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "botgrpc", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)

	st := &store.BotStore{Pool: pool}
	hub := dispatch.NewHub()
	svc := grpcsvc.NewBotGRPC(st, hub)

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	botv1.RegisterBotServiceServer(s, svc)
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		s.Stop()
		_ = conn.Close()
	}
	return botv1.NewBotServiceClient(conn), st, cleanup
}

func withAccount(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	md := metadata.Pairs("x-voice-user-id", accountID.String(), "x-voice-profile-id", profileID.String())
	return metadata.NewOutgoingContext(ctx, md)
}

func TestExecuteSlashInteraction_webhookPong(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := context.Background()
	owner := uuid.New()
	profile := uuid.New()
	ctx = withAccount(ctx, owner, profile)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name:        "PingBot",
		Description: "pong",
		ScopesJson:  `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":"pong","ephemeral":true}`))
	}))
	defer srv.Close()

	manifestYAML := "name: PingBot\ndescription: pong\nwebhook_url: " + srv.URL + "\nscopes: [TEXT_CHAT_SEND_MESSAGES]\ncommands:\n  - name: ping\n    description: ping\n"
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
		BotId:        botID,
		ManifestYaml: manifestYAML,
	})
	require.NoError(t, err)

	chatID := uuid.New()
	spaceID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	installBotInSpaceWithPresence(t, st, ctx, botUUID, spaceID, profile, []uuid.UUID{chatID})

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetContent())
	require.Equal(t, "pong", resp.GetContent())
}

func TestValidateManifest_ping(t *testing.T) {
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	resp, err := client.ValidateManifest(context.Background(), &botv1.ValidateManifestRequest{
		ManifestYaml: `name: PingBot
description: pong
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping`,
	})
	require.NoError(t, err)
	require.True(t, resp.GetValid())
}

func TestExecuteSlashInteraction_timeout(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "SlowBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	// Polling mode: interaction is enqueued but never completed → 3s hub timeout.
	manifestYAML := `name: SlowBot
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
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}, BotId: botID, CommandName: "ping",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetErrorCode())
	require.Equal(t, "bot_timeout", resp.GetErrorCode())
}

func TestExecuteSlashInteraction_notWhitelisted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "PingBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	manifestYAML := `name: PingBot
description: pong
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat:        &chatv1.ChatRef{Id: uuid.New().String(), Type: &chatType},
		BotId:       botID,
		CommandName: "ping",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func authProfile(ctx context.Context) (uuid.UUID, bool) {
	md, _ := metadata.FromOutgoingContext(ctx)
	vals := md.Get("x-voice-profile-id")
	if len(vals) == 0 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(vals[0])
	return id, err == nil
}

func TestEditBotMessage_integration_messagingMockOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClient{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "EditBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botToken := reg.GetTokenResponse().GetToken()

	manifestYAML := `name: EditBot
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

	botCtx := withBotToken(context.Background(), botToken)
	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	sendResp, err := client.SendBotMessage(botCtx, &botv1.SendBotMessageRequest{
		Chat:    &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		Content: "original",
	})
	require.NoError(t, err)
	messageID := sendResp.GetMessage().GetId()
	require.NotEmpty(t, messageID)

	editResp, err := client.EditBotMessage(botCtx, &botv1.EditBotMessageRequest{
		MessageId: messageID,
		Content:   "edited by bot",
	})
	require.NoError(t, err)
	require.Equal(t, "edited by bot", editResp.GetMessage().GetContent())
	require.Equal(t, 1, msg.editCalls)

	msg.editErr = status.Error(codes.PermissionDenied, "not message owner")
	_, err = client.EditBotMessage(botCtx, &botv1.EditBotMessageRequest{
		MessageId: messageID,
		Content:   "should fail",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	msg.editErr = status.Error(codes.NotFound, "message not found")
	_, err = client.EditBotMessage(botCtx, &botv1.EditBotMessageRequest{
		MessageId: uuid.NewString(),
		Content:   "missing",
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))

	badCtx := withBotToken(context.Background(), "vb_invalid")
	_, err = client.EditBotMessage(badCtx, &botv1.EditBotMessageRequest{
		MessageId: messageID,
		Content:   "nope",
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestEditBotMessage_integration_passesBotActorToMessaging(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	msg := &fakeMessagingClientCapturingActor{}
	client, st, cleanup := startBotGRPCWithDeps(t, nil, msg)
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "ActorBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botToken := reg.GetTokenResponse().GetToken()

	botUUID, _ := uuid.Parse(reg.GetBot().GetId())
	row, err := st.GetBotByID(context.Background(), botUUID)
	require.NoError(t, err)

	botCtx := withBotToken(context.Background(), botToken)
	_, err = client.EditBotMessage(botCtx, &botv1.EditBotMessageRequest{
		MessageId: uuid.NewString(),
		Content:   "check actor",
	})
	require.NoError(t, err)
	require.Equal(t, row.ActorProfileID.String(), msg.lastProfileID)
	require.Equal(t, row.OwnerAccountID.String(), msg.lastUserID)
}

type fakeMessagingClientCapturingActor struct {
	fakeMessagingClient
	lastProfileID string
	lastUserID    string
}

func (f *fakeMessagingClientCapturingActor) EditMessage(ctx context.Context, req *messagingv1.EditMessageRequest) (*messagingv1.EditMessageResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-voice-profile-id"); len(vals) > 0 {
			f.lastProfileID = vals[0]
		}
		if vals := md.Get("x-voice-user-id"); len(vals) > 0 {
			f.lastUserID = vals[0]
		}
	}
	return f.fakeMessagingClient.EditMessage(ctx, req)
}

func installBotInSpaceWithPresence(
	t *testing.T,
	st *store.BotStore,
	ctx context.Context,
	botID uuid.UUID,
	spaceID uuid.UUID,
	profile uuid.UUID,
	chats []uuid.UUID,
) {
	t.Helper()
	_, err := st.InstallInSpace(ctx, botID, spaceID, profile, chats)
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botID))
}
