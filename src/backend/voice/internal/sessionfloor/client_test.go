package sessionfloor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

type floorServer struct {
	authv1.UnimplementedAuthServiceServer
	floor int64
	key   *rsa.PublicKey
	seen  bool
	err   error
}

func (s *floorServer) GetVoiceSessionEpochFloor(ctx context.Context,
	request *authv1.GetVoiceSessionEpochFloorRequest) (*authv1.GetVoiceSessionEpochFloorResponse, error) {
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return nil, err
	}
	callHash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: "voice", ExpectedAudience: Audience, ExpectedRPC: RPC,
		ExpectedRequestID: transport.RequestID, ExpectedRequestHash: callHash,
		KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return s.key, nil },
		Clock:       time.Now,
	})
	if err != nil || verified.Subject != "service:voice" {
		return nil, io.ErrUnexpectedEOF
	}
	s.seen = true
	if s.err != nil {
		return nil, s.err
	}
	return &authv1.GetVoiceSessionEpochFloorResponse{SessionEpochFloor: s.floor}, nil
}

func TestRequireCurrentUsesRequestBoundVoiceServicePrincipalAndRejectsStaleFloor(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	serverImpl := &floorServer{floor: 5, key: &key.PublicKey}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	authv1.RegisterAuthServiceServer(server, serverImpl)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///session-floor-test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := New(authv1.NewAuthServiceClient(conn), issuer)
	accountID := "8a78bd68-75e9-4f21-9387-3bdc2f6116ab"
	require.NoError(t, client.RequireCurrent(context.Background(), accountID, 5))
	require.True(t, serverImpl.seen)
	require.Error(t, client.RequireCurrent(context.Background(), accountID, 4))
	require.Error(t, client.RequireCurrent(context.Background(), "not-an-account", 5))
	serverImpl.err = status.Error(codes.Unavailable, "floor lookup unavailable")
	require.Equal(t, codes.Unavailable, status.Code(client.RequireCurrent(context.Background(), accountID, 5)))
}

func TestCanonicalAccountIDRequiresLowercaseCanonicalUUID(t *testing.T) {
	require.True(t, CanonicalAccountID("8a78bd68-75e9-4f21-9387-3bdc2f6116ab"))
	require.False(t, CanonicalAccountID("8A78BD68-75E9-4F21-9387-3BDC2F6116AB"))
	require.False(t, CanonicalAccountID(" 8a78bd68-75e9-4f21-9387-3bdc2f6116ab"))
}
