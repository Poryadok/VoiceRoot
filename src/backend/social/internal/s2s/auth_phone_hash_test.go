package s2s

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	authv1 "voice.app/voice/auth/v1"
)

type stubAuthResolvePhoneHashes struct {
	authv1.UnimplementedAuthServiceServer
	byHash map[string]string
}

func (s *stubAuthResolvePhoneHashes) ResolvePhoneHashes(_ context.Context, req *authv1.ResolvePhoneHashesRequest) (*authv1.ResolvePhoneHashesResponse, error) {
	resp := &authv1.ResolvePhoneHashesResponse{}
	for _, h := range req.GetPhoneHashes() {
		if pid, ok := s.byHash[h]; ok {
			resp.Matches = append(resp.Matches, &authv1.PhoneHashProfileMatch{
				PhoneHash: h,
				ProfileId: pid,
			})
		}
	}
	return resp, nil
}

func startBufconnAuth(t *testing.T, impl authv1.AuthServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	authv1.RegisterAuthServiceServer(srv, impl)
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

func TestGRPCAuthPhoneHashLookup_ProfileIDsByPhoneHashes(t *testing.T) {
	t.Parallel()
	target := uuid.New()
	hash := "sha256-test-phone-hash"
	conn, cleanup := startBufconnAuth(t, &stubAuthResolvePhoneHashes{
		byHash: map[string]string{hash: target.String()},
	})
	t.Cleanup(cleanup)

	lookup := NewGRPCAuthPhoneHashLookup(conn)
	got, err := lookup.ProfileIDsByPhoneHashes(context.Background(), []string{hash, "sha256-missing"})
	require.NoError(t, err)
	require.Equal(t, target, got[hash])
	require.NotContains(t, got, "sha256-missing")
}

func TestGRPCAuthPhoneHashLookup_NilClient(t *testing.T) {
	t.Parallel()
	var lookup *GRPCAuthPhoneHashLookup
	got, err := lookup.ProfileIDsByPhoneHashes(context.Background(), []string{"x"})
	require.NoError(t, err)
	require.Empty(t, got)
}
