package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/messaging/internal/s2s"

	messagingv1 "voice.app/voice/messaging/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

type ownerOnlyUserServer struct {
	userv1.UnimplementedUserServiceServer
	owners map[string]string
}

func (s *ownerOnlyUserServer) ResolveAccountIDForProfile(_ context.Context, req *userv1.ResolveAccountIDForProfileRequest) (*userv1.ResolveAccountIDForProfileResponse, error) {
	owner, ok := s.owners[req.GetProfileId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	return &userv1.ResolveAccountIDForProfileResponse{AccountId: owner}, nil
}

type blockedPairSocialServer struct {
	socialv1.UnimplementedSocialServiceServer
	blockedA, blockedB string
}

func (s *blockedPairSocialServer) IsBlocked(_ context.Context, req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
	return &socialv1.IsBlockedResponse{Blocked: req.GetAccountIdA() == s.blockedA && req.GetAccountIdB() == s.blockedB}, nil
}

func startOwnerGateBufconn(t *testing.T, register func(grpc.ServiceRegistrar, userv1.UserServiceServer), impl userv1.UserServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	register(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return conn, func() { _ = conn.Close(); srv.Stop(); _ = lis.Close() }
}

func startBlockedPairSocialBufconn(t *testing.T, impl socialv1.SocialServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return conn, func() { _ = conn.Close(); srv.Stop(); _ = lis.Close() }
}

// TestMessagingSend_BlockedPairUsesDedicatedProfileOwnerRPC reaches the actual
// User and Social adapters before rejecting the DM write.  The User server
// deliberately implements no public GetProfile RPC.
func TestMessagingSend_BlockedPairUsesDedicatedProfileOwnerRPC(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyDeletedPeerMessagingMigrations(t, ctx, pool)
	chatID, profileA, profileB := uuid.New(), uuid.New(), uuid.New()
	accountA, accountB := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, chatID, profileA, profileB)

	userConn, closeUser := startOwnerGateBufconn(t, userv1.RegisterUserServiceServer, &ownerOnlyUserServer{owners: map[string]string{profileA.String(): accountA.String(), profileB.String(): accountB.String()}})
	t.Cleanup(closeUser)
	socialConn, closeSocial := startBlockedPairSocialBufconn(t, &blockedPairSocialServer{blockedA: accountA.String(), blockedB: accountB.String()})
	t.Cleanup(closeSocial)
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles:               &s2s.UserGRPCProfiles{Client: userv1.NewUserServiceClient(userConn)},
		Blocks:                     s2s.NewSocialGRPCBlocks(socialConn),
		DeletedAccounts:            allowDeletedAccounts{},
		RequireDeletedAccountsSeam: true,
		MessageEvents:              events,
	})

	_, err := client.SendMessage(withProfileCtx(ctx, accountA, profileA), &messagingv1.SendMessageRequest{Chat: chatDMRef(chatID), Content: "blocked", AttachmentsJson: "[]", MentionsJson: "[]"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, messageCountForDeletedPeerTest(t, ctx, pool, chatID))
	require.Zero(t, events.eventCount())
}
