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
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/pkg/socialprincipal"

	userv1 "voice.app/voice/user/v1"
)

// The protected User listener must expose only Social's three signed lookup
// methods; 9090 remains incapable of accepting a raw Social identity.
func TestSocialProtectedListener_ExposesOnlySignedLookupContract(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	RegisterSocialPrivacyServer(srv, &UserGRPC{})
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})
	methods := srv.GetServiceInfo()["voice.user.v1.UserService"].Methods
	got := make([]string, 0, len(methods))
	for _, method := range methods {
		got = append(got, method.Name)
	}
	require.ElementsMatch(t, []string{"GetPrivacySettings", "GetProfile", "ListProfileIDsForAccount"}, got)

	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///social-user", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := userv1.NewUserServiceClient(conn)
	_, err = client.GetProfile(context.Background(), &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: uuid.NewString()}})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.ListProfileIDsForAccount(context.Background(), &userv1.ListProfileIDsForAccountRequest{AccountId: uuid.NewString()})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestOrdinaryUserListener_RejectsRawSocialLookupMetadata(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(socialprincipal.OrdinaryUnaryInterceptor("user")))
	userv1.RegisterUserServiceServer(srv, &UserGRPC{})
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///ordinary-user", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := userv1.NewUserServiceClient(conn)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "social"))
	_, err = client.GetProfile(ctx, &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: "not-a-uuid"}})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.ListProfileIDsForAccount(ctx, &userv1.ListProfileIDsForAccountRequest{AccountId: "not-a-uuid"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
