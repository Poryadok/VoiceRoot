package grpcsvc

import (
	"context"
	"encoding/base64"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// app stack5 red tests: DM-only opt-in E2E flag on ChatService (docs/PLAN.md, docs/features/encryption.md).

func createDMForE2E(
	t *testing.T,
	client chatv1.ChatServiceClient,
	profiles mapProfileAccounts,
	profA, profB uuid.UUID,
) *chatv1.Chat {
	t.Helper()
	ctxA := withAccountProfileCtx(context.Background(), profiles[profA], profA)
	created, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chat := created.GetChat()
	require.NotNil(t, chat)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, chat.GetType())
	return chat
}

// TestEnableChatE2E_DM_Succeeds documents app stack5: both DM members may enable E2E per chat.
func TestEnableChatE2E_DM_Succeeds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profA, profB := uuid.New(), uuid.New()
	profiles := profileMap(profA, profB)

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createDMForE2E(t, client, profiles, profA, profB)

	_, err := client.EnableChatE2E(ctxFor(t, profiles, profA), &chatv1.EnableChatE2ERequest{
		ChatId: chat.GetId(),
	})
	require.NoError(t, err)

	got, err := client.GetChat(ctxFor(t, profiles, profA), &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
	require.True(t, got.GetChat().GetE2EEnabled(), "DM chat should expose e2e_enabled after EnableChatE2E")
}

// TestEnableChatE2E_Group_FailedPrecondition documents E2E is DM-only (encryption.md).
func TestEnableChatE2E_Group_FailedPrecondition(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 3)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	group := createStandaloneGroup(t, client, profiles, owner, "no-e2e-group", inviteeA, inviteeB)

	_, err := client.EnableChatE2E(ctxFor(t, profiles, owner), &chatv1.EnableChatE2ERequest{
		ChatId: group.GetId(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestDisableChatE2E_DM_Succeeds documents opt-out reverts chat to plaintext mode server-side.
func TestDisableChatE2E_DM_Succeeds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profA, profB := uuid.New(), uuid.New()
	profiles := profileMap(profA, profB)

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createDMForE2E(t, client, profiles, profA, profB)
	ctxA := ctxFor(t, profiles, profA)

	_, err := client.EnableChatE2E(ctxA, &chatv1.EnableChatE2ERequest{ChatId: chat.GetId()})
	require.NoError(t, err)

	_, err = client.DisableChatE2E(ctxA, &chatv1.DisableChatE2ERequest{ChatId: chat.GetId()})
	require.NoError(t, err)

	got, err := client.GetChat(ctxA, &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
	require.False(t, got.GetChat().GetE2EEnabled(), "DM chat e2e_enabled should be false after DisableChatE2E")
}

// TestChat_E2EEnabled_ExposedOnGetDM documents Chat.e2e_enabled is returned from GetDM.
func TestChat_E2EEnabled_ExposedOnGetDM(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profA, profB := uuid.New(), uuid.New()
	profiles := profileMap(profA, profB)

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createDMForE2E(t, client, profiles, profA, profB)
	_, err := client.EnableChatE2E(ctxFor(t, profiles, profA), &chatv1.EnableChatE2ERequest{
		ChatId: chat.GetId(),
	})
	require.NoError(t, err)

	got, err := client.GetDM(ctxFor(t, profiles, profB), &chatv1.GetDMRequest{OtherProfileId: profA.String()})
	require.NoError(t, err)
	require.True(t, got.GetChat().GetE2EEnabled())
}

func samplePreKeyBundleB64() string {
	return base64.StdEncoding.EncodeToString([]byte("phase15-test-prekey-bundle-v1"))
}

// e2ePreKeyMessagingStub is a minimal Messaging pre-key directory for Chat E2E gate tests.
type e2ePreKeyMessagingStub struct {
	messagingv1.UnimplementedMessagingServiceServer
	bundles  map[string]string
	getCalls []string
}

func (s *e2ePreKeyMessagingStub) GetPreKeyBundle(_ context.Context, req *messagingv1.GetPreKeyBundleRequest) (*messagingv1.GetPreKeyBundleResponse, error) {
	profileID := req.GetProfileId()
	s.getCalls = append(s.getCalls, profileID)
	bundle, ok := s.bundles[profileID]
	if !ok || bundle == "" {
		return nil, status.Error(codes.NotFound, "pre-key bundle not found")
	}
	return &messagingv1.GetPreKeyBundleResponse{Bundle: bundle}, nil
}

func seedStubPreKeyBundle(stub *e2ePreKeyMessagingStub, profileID uuid.UUID) {
	if stub.bundles == nil {
		stub.bundles = map[string]string{}
	}
	stub.bundles[profileID.String()] = samplePreKeyBundleB64()
}

func startE2EPreKeyMessagingClient(t *testing.T, srv messagingv1.MessagingServiceServer) (messagingv1.MessagingServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	grpcSrv := grpc.NewServer()
	messagingv1.RegisterMessagingServiceServer(grpcSrv, srv)
	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			t.Logf("messaging grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return messagingv1.NewMessagingServiceClient(conn), func() {
		_ = conn.Close()
		grpcSrv.Stop()
	}
}

// WithMessagingE2EClient wires Messaging pre-key lookup for EnableChatE2E gate tests.
func WithMessagingE2EClient(c messagingv1.MessagingServiceClient) chatServerOption {
	return func(svc *ChatGRPC) {
		svc.E2EPreKeyGate = NewMessagingE2EPreKeyGate(c)
	}
}

// TestEnableChatE2E_FailsWhenPeerMissingPreKey documents both DM peers need pre-keys before enable.
func TestEnableChatE2E_FailsWhenPeerMissingPreKey(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profA, profB := uuid.New(), uuid.New()
	profiles := profileMap(profA, profB)

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)

	stub := &e2ePreKeyMessagingStub{bundles: map[string]string{}}
	seedStubPreKeyBundle(stub, profA)
	msgClient, cleanupMsg := startE2EPreKeyMessagingClient(t, stub)
	t.Cleanup(cleanupMsg)

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithMessagingE2EClient(msgClient))
	t.Cleanup(cleanup)

	chat := createDMForE2E(t, client, profiles, profA, profB)

	_, err := client.EnableChatE2E(ctxFor(t, profiles, profA), &chatv1.EnableChatE2ERequest{
		ChatId: chat.GetId(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestEnableChatE2E_SucceedsWhenBothHavePreKeys documents happy path with Messaging pre-key gate.
func TestEnableChatE2E_SucceedsWhenBothHavePreKeys(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profA, profB := uuid.New(), uuid.New()
	profiles := profileMap(profA, profB)

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)

	stub := &e2ePreKeyMessagingStub{bundles: map[string]string{}}
	seedStubPreKeyBundle(stub, profA)
	seedStubPreKeyBundle(stub, profB)
	msgClient, cleanupMsg := startE2EPreKeyMessagingClient(t, stub)
	t.Cleanup(cleanupMsg)

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithMessagingE2EClient(msgClient))
	t.Cleanup(cleanup)

	chat := createDMForE2E(t, client, profiles, profA, profB)

	_, err := client.EnableChatE2E(ctxFor(t, profiles, profA), &chatv1.EnableChatE2ERequest{
		ChatId: chat.GetId(),
	})
	require.NoError(t, err)

	got, err := client.GetChat(ctxFor(t, profiles, profA), &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
	require.True(t, got.GetChat().GetE2EEnabled())

	require.ElementsMatch(t, []string{profA.String(), profB.String()}, stub.getCalls,
		"EnableChatE2E must verify both peers have pre-keys via Messaging")
}
