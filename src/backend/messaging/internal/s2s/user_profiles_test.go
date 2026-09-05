package s2s

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

	userv1 "voice.app/voice/user/v1"
)

type stubUserProfileOwner struct {
	userv1.UnimplementedUserServiceServer
	accountID string
	err       error
}

type stubUserEmptyProfileOwner struct {
	userv1.UnimplementedUserServiceServer
}

func (s *stubUserEmptyProfileOwner) ResolveAccountIDForProfile(context.Context, *userv1.ResolveAccountIDForProfileRequest) (*userv1.ResolveAccountIDForProfileResponse, error) {
	return &userv1.ResolveAccountIDForProfileResponse{}, nil
}

func (s *stubUserProfileOwner) ResolveAccountIDForProfile(_ context.Context, req *userv1.ResolveAccountIDForProfileRequest) (*userv1.ResolveAccountIDForProfileResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.accountID == "" {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	return &userv1.ResolveAccountIDForProfileResponse{AccountId: s.accountID}, nil
}

func startBufconnUser(t *testing.T, impl userv1.UserServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func TestUserGRPCProfiles_AccountIDByProfileID(t *testing.T) {
	t.Parallel()

	profileID := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	accountID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()
		var u *UserGRPCProfiles
		_, err := u.AccountIDByProfileID(context.Background(), profileID)
		require.Error(t, err)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnUser(t, &stubUserProfileOwner{
			accountID: accountID.String(),
		})
		t.Cleanup(cleanup)
		u := &UserGRPCProfiles{Client: userv1.NewUserServiceClient(conn)}
		got, err := u.AccountIDByProfileID(context.Background(), profileID)
		require.NoError(t, err)
		require.Equal(t, accountID, got)
	})

	t.Run("missing account_id in response", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnUser(t, &stubUserEmptyProfileOwner{})
		t.Cleanup(cleanup)
		u := &UserGRPCProfiles{Client: userv1.NewUserServiceClient(conn)}
		_, err := u.AccountIDByProfileID(context.Background(), profileID)
		require.Error(t, err)
		require.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("non-notfound grpc error", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnUser(t, &stubUserProfileOwner{
			err: status.Error(codes.Unavailable, "user down"),
		})
		t.Cleanup(cleanup)
		u := &UserGRPCProfiles{Client: userv1.NewUserServiceClient(conn)}
		_, err := u.AccountIDByProfileID(context.Background(), profileID)
		require.Error(t, err)
		require.Equal(t, codes.Unavailable, status.Code(err))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnUser(t, &stubUserProfileOwner{err: status.Error(codes.NotFound, "missing")})
		t.Cleanup(cleanup)
		u := &UserGRPCProfiles{Client: userv1.NewUserServiceClient(conn)}
		_, err := u.AccountIDByProfileID(context.Background(), profileID)
		require.Error(t, err)
		require.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("invalid account_id", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnUser(t, &stubUserProfileOwner{
			accountID: "not-a-uuid",
		})
		t.Cleanup(cleanup)
		u := &UserGRPCProfiles{Client: userv1.NewUserServiceClient(conn)}
		_, err := u.AccountIDByProfileID(context.Background(), profileID)
		require.Error(t, err)
		require.Equal(t, codes.Internal, status.Code(err))
	})
}
